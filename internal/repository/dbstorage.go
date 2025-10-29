package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Schera-ole/loyalty_system/internal/auth"
	apperrors "github.com/Schera-ole/loyalty_system/internal/error"
	"github.com/Schera-ole/loyalty_system/internal/model"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type DBStorage struct {
	db *sql.DB
}

func NewDBStorage(dsn string) (*DBStorage, error) {
	dbConnect, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	return &DBStorage{db: dbConnect}, nil
}

func (storage *DBStorage) Close() error {
	return storage.db.Close()
}

func (storage *DBStorage) SetUser(ctx context.Context, user model.User) error {
	// Validate input
	if user.Username == "" || user.Password == "" {
		return apperrors.ErrInvalidRequest
	}

	tx, err := storage.db.Begin()
	if err != nil {
		return fmt.Errorf("can't start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	// Check if user already exists before attempting to create
	exists, err := storage.checkUserExists(ctx, tx, user.Username)
	if err != nil {
		return fmt.Errorf("error checking user existence: %w", err)
	}
	if exists {
		return apperrors.ErrUserAlreadyExists
	}

	passwordHash, err := auth.HashPassword(user.Password)
	if err != nil {
		return apperrors.ErrPasswordHashing
	}

	// Insert user
	query := "INSERT INTO users (username, password_hash, created_at, updated_at) VALUES ($1, $2, NOW(), NOW()) RETURNING id"
	var userID string
	err = tx.QueryRowContext(ctx, query, user.Username, passwordHash).Scan(&userID)
	if err != nil {
		return fmt.Errorf("error saving user: %w", err)
	}

	// Create user balance record with default values
	balanceQuery := "INSERT INTO user_balance (user_id, balance, total_spent, updated_at) VALUES ($1, 0, 0, NOW())"
	_, err = tx.Exec(balanceQuery, userID)
	if err != nil {
		return fmt.Errorf("error creating user balance: %w", err)
	}

	return nil
}

func (storage *DBStorage) CheckUser(ctx context.Context, user model.User) (bool, error) {
	// Validate input
	if user.Username == "" || user.Password == "" {
		return false, apperrors.ErrInvalidCredentials
	}

	// Check if user exists and get password hash in a single query
	var storedHash string
	query := "SELECT password_hash FROM users WHERE username = $1"
	err := storage.db.QueryRowContext(ctx, query, user.Username).Scan(&storedHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, apperrors.ErrUserNotFound
		}
		return false, fmt.Errorf("error retrieving user: %w", err)
	}

	// Verify the password
	err = auth.CheckPassword(user.Password, storedHash)
	if err != nil {
		return false, apperrors.ErrInvalidPassword
	}

	return true, nil
}

func (storage *DBStorage) GetOrders(ctx context.Context, username string) ([]model.Order, error) {
	var orders []model.Order
	query := `
		SELECT o.order_number, o.status, o.accrual, o.uploaded_at
		FROM orders o
		INNER JOIN users u ON o.user_id = u.id
		WHERE u.username = $1
		ORDER BY o.uploaded_at DESC
	`
	rows, err := storage.db.QueryContext(ctx, query, username)
	if err != nil {
		return orders, fmt.Errorf("error executing query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var order model.Order
		err := rows.Scan(
			&order.Number,
			&order.Status,
			&order.Accrual,
			&order.UploadedAt,
		)
		if err != nil {
			return orders, fmt.Errorf("error scanning row: %w", err)
		}
		orders = append(orders, order)
	}

	if err = rows.Err(); err != nil {
		return orders, fmt.Errorf("error iterating rows: %w", err)
	}

	return orders, nil
}

func (storage *DBStorage) GetUserBalance(ctx context.Context, username string) (model.UserBalance, error) {
	var userBalance model.UserBalance
	query := `
		SELECT ub.balance, ub.total_spent
		FROM user_balance ub
		INNER JOIN users u ON ub.user_id = u.id
		WHERE u.username = $1
	`
	err := storage.db.QueryRowContext(ctx, query, username).Scan(&userBalance.Balance, &userBalance.TotalSpent)
	if err != nil {
		if err == sql.ErrNoRows {
			return userBalance, apperrors.ErrBalanceNotFound
		}
		return userBalance, fmt.Errorf("error retrieving balance: %w", err)
	}
	return userBalance, nil
}

func (storage *DBStorage) GetWithdrawals(ctx context.Context, username string) ([]model.Withdrawal, error) {
	var withdrawals []model.Withdrawal
	query := `
		SELECT lt.order_number, lt.points, lt.processed_at
		FROM loyalty_transactions lt
		INNER JOIN users u ON lt.user_id = u.id
		WHERE u.username = $1 AND lt.transaction_type = 'spend'
		ORDER BY lt.processed_at DESC
	`
	rows, err := storage.db.QueryContext(ctx, query, username)
	if err != nil {
		return withdrawals, fmt.Errorf("error executing query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var withdrawal model.Withdrawal
		err := rows.Scan(
			&withdrawal.Order,
			&withdrawal.Sum,
			&withdrawal.ProcessedAt,
		)
		if err != nil {
			return withdrawals, fmt.Errorf("error scanning row: %w", err)
		}
		withdrawals = append(withdrawals, withdrawal)
	}

	if err = rows.Err(); err != nil {
		return withdrawals, fmt.Errorf("error iterating rows: %w", err)
	}

	return withdrawals, nil
}

func (storage *DBStorage) SpendPoints(ctx context.Context, orderWithdrawal model.OrderWithdrawal) error {
	var currentBalance float64
	var userID string

	tx, err := storage.db.Begin()
	if err != nil {
		return fmt.Errorf("can't start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	// Get user ID and current balance
	queryUserBalance := `
		SELECT u.id, ub.balance
		FROM user_balance ub
		INNER JOIN users u ON ub.user_id = u.id
		WHERE u.username = $1
	`
	err = tx.QueryRowContext(ctx, queryUserBalance, orderWithdrawal.User).Scan(&userID, &currentBalance)
	if err != nil {
		if err == sql.ErrNoRows {
			return apperrors.ErrBalanceNotFound
		}
		return fmt.Errorf("error checking user balance: %w", err)
	}

	// Check if user has sufficient balance
	if float64(orderWithdrawal.Sum) > currentBalance {
		return apperrors.ErrInsufficientFunds
	}

	// Update user balance
	updateBalanceQuery := `
		UPDATE user_balance
		SET balance = balance - $1, total_spent = total_spent + $1, updated_at = NOW()
		WHERE user_id = $2
	`
	_, err = tx.ExecContext(ctx, updateBalanceQuery, float64(orderWithdrawal.Sum), userID)
	if err != nil {
		return fmt.Errorf("error updating user balance: %w", err)
	}

	// Create loyalty transaction record
	insertTransactionQuery := `
		INSERT INTO loyalty_transactions (user_id, order_number, points, transaction_type, processed_at)
		VALUES ($1, $2, $3, 'spend', NOW())
	`
	_, err = tx.ExecContext(ctx, insertTransactionQuery, userID, orderWithdrawal.Order, float64(orderWithdrawal.Sum))
	if err != nil {
		return fmt.Errorf("error creating loyalty transaction: %w", err)
	}

	return nil
}

// AddOrder adds a new order for a user
func (storage *DBStorage) AddOrder(ctx context.Context, username string, orderNumber string) error {
	tx, err := storage.db.Begin()
	if err != nil {
		return fmt.Errorf("can't start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	// Get user ID
	var userID string
	query := "SELECT id FROM users WHERE username = $1"
	err = tx.QueryRowContext(ctx, query, username).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return apperrors.ErrUserNotFound
		}
		return fmt.Errorf("error getting user ID: %w", err)
	}

	// Check if order already exists for any user
	var existingUserID string
	checkOrderQuery := "SELECT user_id FROM orders WHERE order_number = $1"
	err = tx.QueryRowContext(ctx, checkOrderQuery, orderNumber).Scan(&existingUserID)
	if err == nil {
		// Order exists, check if it's for the same user
		if existingUserID == userID {
			return apperrors.ErrOrderAlreadyExists
		}
		// Order exists for different user
		return apperrors.ErrOrderOwnedByAnotherUser
	} else if err != sql.ErrNoRows {
		return fmt.Errorf("error checking order existence: %w", err)
	}

	// Insert order
	insertQuery := "INSERT INTO orders (order_number, user_id, status, uploaded_at) VALUES ($1, $2, 'NEW', NOW())"
	_, err = tx.ExecContext(ctx, insertQuery, orderNumber, userID)
	if err != nil {
		return fmt.Errorf("error inserting order: %w", err)
	}

	return nil
}

// UpdateOrderStatus updates the status of an order
func (storage *DBStorage) UpdateOrderStatus(ctx context.Context, orderNumber string, status string) error {
	query := "UPDATE orders SET status = $1, updated_at = NOW() WHERE order_number = $2"
	_, err := storage.db.ExecContext(ctx, query, status, orderNumber)
	if err != nil {
		return fmt.Errorf("error updating order status: %w", err)
	}
	return nil
}

// UpdateOrderStatusAndAccrual updates the status and accrual of an order
func (storage *DBStorage) UpdateOrderStatusAndAccrual(ctx context.Context, orderNumber string, status string, accrual *float64) error {
	tx, err := storage.db.Begin()
	if err != nil {
		return fmt.Errorf("can't start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	// Get user ID for the order
	var userID string
	getUserQuery := "SELECT user_id FROM orders WHERE order_number = $1"
	err = tx.QueryRowContext(ctx, getUserQuery, orderNumber).Scan(&userID)
	if err != nil {
		return fmt.Errorf("error getting user ID for order: %w", err)
	}

	// Update order status and accrual
	updateOrderQuery := "UPDATE orders SET status = $1, accrual = $2, updated_at = NOW() WHERE order_number = $3"
	_, err = tx.ExecContext(ctx, updateOrderQuery, status, accrual, orderNumber)
	if err != nil {
		return fmt.Errorf("error updating order status and accrual: %w", err)
	}

	// If order is processed with accrual, update user balance and create transaction
	if status == "PROCESSED" && accrual != nil && *accrual > 0 {
		// Update user balance
		updateBalanceQuery := `
			UPDATE user_balance 
			SET balance = balance + $1, updated_at = NOW() 
			WHERE user_id = $2
		`
		_, err = tx.ExecContext(ctx, updateBalanceQuery, *accrual, userID)
		if err != nil {
			return fmt.Errorf("error updating user balance: %w", err)
		}

		// Create loyalty transaction record for earned points
		insertTransactionQuery := `
			INSERT INTO loyalty_transactions (user_id, order_number, points, transaction_type, processed_at)
			VALUES ($1, $2, $3, 'earn', NOW())
		`
		_, err = tx.ExecContext(ctx, insertTransactionQuery, userID, orderNumber, *accrual)
		if err != nil {
			return fmt.Errorf("error creating loyalty transaction: %w", err)
		}
	}

	return nil
}

// Ping checks if the database connection is alive
func (storage *DBStorage) Ping(ctx context.Context) error {
	return storage.db.PingContext(ctx)
}

// Check if user has already existed
func (storage *DBStorage) checkUserExists(ctx context.Context, tx *sql.Tx, username string) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)"
	err := tx.QueryRowContext(ctx, query, username).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error checking if user exists: %w", err)
	}
	return exists, nil
}

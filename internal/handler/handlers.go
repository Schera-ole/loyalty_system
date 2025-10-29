package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Schera-ole/loyalty_system/internal/config"
	apperrors "github.com/Schera-ole/loyalty_system/internal/error"
	appmiddleware "github.com/Schera-ole/loyalty_system/internal/middleware"
	"github.com/Schera-ole/loyalty_system/internal/model"
	"github.com/Schera-ole/loyalty_system/internal/service"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/jwtauth/v5"
	"go.uber.org/zap"
)

func Router(
	logger *zap.SugaredLogger,
	config *config.SystemConfig,
	LSService *service.LoyaltySystemService,
) chi.Router {
	router := chi.NewRouter()
	router.Use(middleware.StripSlashes)
	router.Use(appmiddleware.LoggingMiddleware(logger))
	router.Use(appmiddleware.GzipMiddleware)
	router.Use(middleware.Timeout(15 * time.Second))

	// JWT token authentication setup
	tokenAuth := jwtauth.New(config.JwtAlgorithm, []byte(config.JwtSecretKey), nil)

	// Public routes
	router.Group(func(r chi.Router) {
		r.Post("/api/user/register", func(w http.ResponseWriter, r *http.Request) {
			SignUpHandler(w, r, logger, LSService, tokenAuth)
		})
		r.Post("/api/user/login", func(w http.ResponseWriter, r *http.Request) {
			SignInHandler(w, r, logger, LSService, tokenAuth)
		})
	})

	// Protected routes - require JWT authentication
	router.Group(func(r chi.Router) {
		// JWT middleware - verifies token from Authorization header
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(jwtauth.Authenticator(tokenAuth))

		r.Post("/api/user/orders", func(w http.ResponseWriter, r *http.Request) {
			SendOrderHandler(w, r, logger, LSService, config)
		})
		r.Get("/api/user/orders", func(w http.ResponseWriter, r *http.Request) {
			GetOrdersHandler(w, r, logger, LSService)
		})
		r.Get("/api/user/balance", func(w http.ResponseWriter, r *http.Request) {
			GetBalanceHandler(w, r, logger, LSService)
		})
		r.Post("/api/user/balance/withdraw", func(w http.ResponseWriter, r *http.Request) {
			WithdrawPointsHandler(w, r, logger, LSService)
		})
		r.Get("/api/user/withdrawals", func(w http.ResponseWriter, r *http.Request) {
			GetWithdrawalsHandler(w, r, logger, LSService)
		})
	})

	return router
}

func SendOrderHandler(w http.ResponseWriter, r *http.Request, logger *zap.SugaredLogger, lss *service.LoyaltySystemService, config *config.SystemConfig) {
	body, err := HandleDecompression(r)
	if err != nil {
		http.Error(w, "Failed to decompress request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	orderNumber := string(body)
	if len(orderNumber) == 0 {
		http.Error(w, "Empty order number", http.StatusUnprocessableEntity)
		return
	}

	// Validate using Luhn algorithm
	if !isValidLuhn(orderNumber) {
		http.Error(w, "Invalid order number format", http.StatusUnprocessableEntity)
		return
	}

	// Get username from context (JWT token)
	_, claims, _ := jwtauth.FromContext(r.Context())
	username, ok := claims["user_id"].(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	logger.Infow("Order received", "username", username, "order_number", orderNumber)

	ctx := r.Context()

	// Add order to database
	err = lss.AddOrder(ctx, username, orderNumber)
	if err != nil {
		// Handle specific errors with appropriate HTTP status codes
		switch {
		case errors.Is(err, apperrors.ErrUserNotFound):
			logger.Errorw("User not found", "username", username, "error", err)
			http.Error(w, "User not found", http.StatusInternalServerError)
		case errors.Is(err, apperrors.ErrOrderAlreadyExists):
			logger.Warnw("Order already exists", "username", username, "order_number", orderNumber)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Order already uploaded by this user"))
			return
		case errors.Is(err, apperrors.ErrOrderOwnedByAnotherUser):
			logger.Warnw("Order already exists for another user", "username", username, "order_number", orderNumber)
			http.Error(w, "Order already exists", http.StatusConflict)
			return
		default:
			logger.Errorw("Failed to add order", "username", username, "order_number", orderNumber, "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Start polling the external accrual system for order status
	lss.PollOrderStatus(ctx, orderNumber, config.AccrualAddress)

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("Order accepted for processing"))
}

func SignUpHandler(w http.ResponseWriter, r *http.Request, logger *zap.SugaredLogger, lss *service.LoyaltySystemService, tokenAuth *jwtauth.JWTAuth) {
	// Handle decompression
	body, err := HandleDecompression(r)
	if err != nil {
		http.Error(w, "Failed to decompress request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	var user model.User
	if err := json.Unmarshal(body, &user); err != nil {
		logger.Errorw("Failed to decode user registration request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if user.Username == "" || user.Password == "" {
		logger.Errorw("Invalid registration attempt - missing credentials", "username", user.Username != "", "password", user.Password != "")
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Register the user - SetUser now handles existence checking internally
	if err := lss.SetUser(ctx, user); err != nil {
		// Handle specific errors with appropriate HTTP status codes
		switch {
		case errors.Is(err, apperrors.ErrUserAlreadyExists):
			logger.Warnw("Registration attempt for existing user", "username", user.Username)
			http.Error(w, "User already exists", http.StatusConflict)
		case errors.Is(err, apperrors.ErrInvalidRequest):
			logger.Warnw("Invalid registration request", "username", user.Username)
			http.Error(w, "Invalid request format", http.StatusBadRequest)
		case errors.Is(err, apperrors.ErrPasswordHashing):
			logger.Errorw("Password hashing failed during registration", "username", user.Username, "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		default:
			logger.Errorw("Failed to register user", "username", user.Username, "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Generate JWT token
	_, tokenString, err := tokenAuth.Encode(map[string]interface{}{"user_id": user.Username})
	if err != nil {
		logger.Errorw("Failed to generate JWT token", "username", user.Username, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set Authorization header with Bearer token
	w.Header().Set("Authorization", "Bearer "+tokenString)

	// Also set the token as a cookie for additional compatibility
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    tokenString,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
	})

	// Return successful response with token
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(tokenString); err != nil {
		logger.Errorw("Failed to encode registration response", "username", user.Username, "error", err)
	}
}

func SignInHandler(w http.ResponseWriter, r *http.Request, logger *zap.SugaredLogger, lss *service.LoyaltySystemService, tokenAuth *jwtauth.JWTAuth) {
	// Handle decompression
	body, err := HandleDecompression(r)
	if err != nil {
		http.Error(w, "Failed to decompress request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	var user model.User
	if err := json.Unmarshal(body, &user); err != nil {
		logger.Errorw("Failed to decode user login request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if user.Username == "" || user.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Check user credentials
	_, err = lss.CheckUser(ctx, user)
	if err != nil {
		// Check for specific authentication errors
		if errors.Is(err, apperrors.ErrUserNotFound) || errors.Is(err, apperrors.ErrInvalidPassword) {
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
			return
		}

		// Handle other errors
		logger.Errorw("Failed to authenticate user", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Generate JWT token
	_, tokenString, err := tokenAuth.Encode(map[string]interface{}{"user_id": user.Username})
	if err != nil {
		logger.Errorw("Failed to generate JWT token", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set Authorization header with Bearer token
	w.Header().Set("Authorization", "Bearer "+tokenString)

	// Also set the token as a cookie for additional compatibility
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    tokenString,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
	})

	// Return the token
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(tokenString)
}

func GetOrdersHandler(w http.ResponseWriter, r *http.Request, logger *zap.SugaredLogger, lss *service.LoyaltySystemService) {
	_, claims, _ := jwtauth.FromContext(r.Context())
	username, ok := claims["user_id"].(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	orders, err := lss.GetOrders(ctx, username)
	if err != nil {
		logger.Errorw("Failed to get orders", "username", username, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// If no orders found, return 204 No Content
	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(orders); err != nil {
		logger.Errorw("Failed to encode orders response", "username", username, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func GetBalanceHandler(w http.ResponseWriter, r *http.Request, logger *zap.SugaredLogger, lss *service.LoyaltySystemService) {
	_, claims, _ := jwtauth.FromContext(r.Context())
	username, ok := claims["user_id"].(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	balance, err := lss.GetUserBalance(ctx, username)
	if err != nil {
		logger.Errorw("Failed to get balance", "username", username, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Convert int values to float as per specification
	response := map[string]float64{
		"current":   float64(balance.Balance),
		"withdrawn": float64(balance.TotalSpent),
	}
	json.NewEncoder(w).Encode(response)
}

func WithdrawPointsHandler(w http.ResponseWriter, r *http.Request, logger *zap.SugaredLogger, lss *service.LoyaltySystemService) {
	_, claims, _ := jwtauth.FromContext(r.Context())
	username, ok := claims["user_id"].(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	ctx := r.Context()

	// Handle decompression
	body, err := HandleDecompression(r)
	if err != nil {
		http.Error(w, "Failed to decompress request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	var withdrawal model.Withdrawal
	if err := json.Unmarshal(body, &withdrawal); err != nil {
		logger.Errorw("Failed to decode withdrawal request", "username", username, "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if withdrawal.Order == "" || withdrawal.Sum <= 0 {
		logger.Warnw("Invalid withdrawal request", "username", username, "order", withdrawal.Order, "sum", withdrawal.Sum)
		http.Error(w, "Invalid order number or sum", http.StatusBadRequest)
		return
	}

	if !isValidLuhn(withdrawal.Order) {
		logger.Warnw("Invalid order number format", "username", username, "order", withdrawal.Order)
		http.Error(w, "Invalid order number format", http.StatusUnprocessableEntity) // 422
		return
	}

	// Create OrderWithdrawal model for service
	orderWithdrawal := model.OrderWithdrawal{
		User:  &username,
		Order: withdrawal.Order,
		Sum:   withdrawal.Sum,
	}

	logger.Infow("Processing withdrawal", "username", username, "order", withdrawal.Order, "sum", withdrawal.Sum)

	// Process withdrawal through service
	err = lss.SpendPoints(ctx, orderWithdrawal)
	if err != nil {
		// Handle specific errors with appropriate HTTP status codes
		switch {
		case errors.Is(err, apperrors.ErrInsufficientFunds):
			logger.Warnw("Insufficient funds for withdrawal", "username", username, "order", withdrawal.Order, "sum", withdrawal.Sum)
			http.Error(w, "Insufficient funds", http.StatusPaymentRequired) // 402
		case errors.Is(err, apperrors.ErrBalanceNotFound):
			logger.Errorw("User balance not found", "username", username, "error", err)
			http.Error(w, "User balance not found", http.StatusInternalServerError)
		default:
			logger.Errorw("Failed to process withdrawal", "username", username, "order", withdrawal.Order, "error", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	logger.Infow("Withdrawal processed successfully", "username", username, "order", withdrawal.Order, "sum", withdrawal.Sum)
	w.WriteHeader(http.StatusOK)
}

func GetWithdrawalsHandler(w http.ResponseWriter, r *http.Request, logger *zap.SugaredLogger, lss *service.LoyaltySystemService) {
	_, claims, _ := jwtauth.FromContext(r.Context())
	username, ok := claims["user_id"].(string)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	withdrawals, err := lss.GetWithdrawals(ctx, username)
	if err != nil {
		logger.Errorw("Failed to get withdrawals", "username", username, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// If no withdrawals found, return 204 No Content
	if len(withdrawals) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(withdrawals); err != nil {
		logger.Errorw("Failed to encode withdrawals response", "username", username, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

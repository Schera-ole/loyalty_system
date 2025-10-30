package repository

import (
	"context"

	"github.com/Schera-ole/loyalty_system/internal/model"
)

type Repository interface {
	SetUser(ctx context.Context, user model.User) error
	CheckUser(ctx context.Context, user model.User) (bool, error)
	AddOrder(ctx context.Context, username string, orderNumber string) error
	UpdateOrderStatus(ctx context.Context, orderNumber string, status string) error
	UpdateOrderStatusAndAccrual(ctx context.Context, orderNumber string, status string, accrual *float64) error
	GetOrders(ctx context.Context, username string) ([]model.Order, error)
	GetUserBalance(ctx context.Context, username string) (model.UserBalance, error)
	GetWithdrawals(ctx context.Context, username string) ([]model.Withdrawal, error)
	SpendPoints(ctx context.Context, orderWithdrawal model.OrderWithdrawal) error
	Ping(ctx context.Context) error
	Close() error
}

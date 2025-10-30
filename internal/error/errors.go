package errors

import "errors"

var (
	ErrUserNotFound            = errors.New("user not found")
	ErrInvalidPassword         = errors.New("invalid password")
	ErrUserAlreadyExists       = errors.New("user already exists")
	ErrOrderAlreadyExists      = errors.New("order already exists")
	ErrOrderOwnedByAnotherUser = errors.New("order already exists for another user")
	ErrInvalidCredentials      = errors.New("invalid username or password")
	ErrInvalidRequest          = errors.New("invalid request format")
	ErrPasswordHashing         = errors.New("failed to hash password")
	ErrDatabaseOperation       = errors.New("database operation failed")
	ErrBalanceNotFound         = errors.New("user balance not found")
	ErrInsufficientFunds       = errors.New("insufficient funds")
)

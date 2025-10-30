package model

import "time"

type User struct {
	Username string `json:"login"`
	Password string `json:"password"`
}

type Order struct {
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Accrual    *float64  `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type UserBalance struct {
	Balance    float64 `json:"current"`
	TotalSpent float64 `json:"withdrawn"`
}

type Withdrawal struct {
	Order       string    `json:"order"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}
type OrderWithdrawal struct {
	User  *string `json:"user,omitempty"`
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

type AccrualResponse struct {
	Order   string   `json:"order"`
	Status  string   `json:"status"`
	Accrual *float64 `json:"accrual,omitempty"`
}

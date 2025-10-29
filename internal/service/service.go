package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/Schera-ole/loyalty_system/internal/model"
	"github.com/Schera-ole/loyalty_system/internal/repository"
	"go.uber.org/zap"
)

type LoyaltySystemService struct {
	repo   repository.Repository
	logger *zap.SugaredLogger
}

func NewLoyaltySystemService(repo repository.Repository, logger *zap.SugaredLogger) *LoyaltySystemService {
	return &LoyaltySystemService{repo: repo, logger: logger}
}

func (lss *LoyaltySystemService) SetUser(ctx context.Context, user model.User) error {
	return lss.repo.SetUser(ctx, user)
}

func (lss *LoyaltySystemService) CheckUser(ctx context.Context, user model.User) (bool, error) {
	return lss.repo.CheckUser(ctx, user)
}

func (lss *LoyaltySystemService) GetOrders(ctx context.Context, username string) ([]model.Order, error) {
	return lss.repo.GetOrders(ctx, username)
}

func (lss *LoyaltySystemService) GetUserBalance(ctx context.Context, username string) (model.UserBalance, error) {
	return lss.repo.GetUserBalance(ctx, username)
}

func (lss *LoyaltySystemService) SpendPoints(ctx context.Context, orderWithdrawal model.OrderWithdrawal) error {
	return lss.repo.SpendPoints(ctx, orderWithdrawal)
}

func (lss *LoyaltySystemService) GetWithdrawals(ctx context.Context, username string) ([]model.Withdrawal, error) {
	return lss.repo.GetWithdrawals(ctx, username)
}

func (lss *LoyaltySystemService) AddOrder(ctx context.Context, username string, orderNumber string) error {
	return lss.repo.AddOrder(ctx, username, orderNumber)
}

func (lss *LoyaltySystemService) UpdateOrderStatus(ctx context.Context, orderNumber string, status string) error {
	return lss.repo.UpdateOrderStatus(ctx, orderNumber, status)
}

func (lss *LoyaltySystemService) UpdateOrderStatusAndAccrual(ctx context.Context, orderNumber string, status string, accrual *float64) error {
	return lss.repo.UpdateOrderStatusAndAccrual(ctx, orderNumber, status, accrual)
}

// PollOrderStatus polls the external accrual system for order status until it reaches a final state
// Final states are: PROCESSED (with optional accrual) and INVALID
// Intermediate states are: NEW and PROCESSING
// The function uses a goroutine to avoid blocking the main thread and polls every 5 seconds
// When receiving a 429 (Too Many Requests) response, it respects the Retry-After header or waits 30 seconds
func (lss *LoyaltySystemService) PollOrderStatus(ctx context.Context, orderNumber string, accrualAddress string) {
	go func() {
		// Create a new context for the polling goroutine to avoid cancellation from parent context
		pollCtx := context.Background()

		ticker := time.NewTicker(5 * time.Second) // Poll every 5 seconds
		defer ticker.Stop()

		for {
			select {
			case <-pollCtx.Done():
				return
			case <-ticker.C:
				accrualURL := accrualAddress + "/api/orders/" + orderNumber
				resp, err := http.Get(accrualURL)
				if err != nil {
					// Log error but continue polling
					if lss.logger != nil {
						lss.logger.Errorw("Error making request to accrual system", "error", err, "order", orderNumber)
					}
					continue
				}

				// Parse response to get status
				var accrualResponse model.AccrualResponse
				switch resp.StatusCode {
				case http.StatusOK:
					if err := json.NewDecoder(resp.Body).Decode(&accrualResponse); err != nil {
						resp.Body.Close()
						if lss.logger != nil {
							lss.logger.Errorw("Error decoding accrual response", "error", err, "order", orderNumber)
						}
						continue
					}
					resp.Body.Close()

					// Log successful response
					if lss.logger != nil {
						lss.logger.Infow("Received accrual response", "order", orderNumber, "status", accrualResponse.Status)
					}

					// Check if status is final
					if accrualResponse.Status == "PROCESSED" || accrualResponse.Status == "INVALID" {
						// If processed with accrual, update both status and accrual
						if accrualResponse.Status == "PROCESSED" && accrualResponse.Accrual != nil {
							// Update both status and accrual
							if err := lss.UpdateOrderStatusAndAccrual(pollCtx, orderNumber, accrualResponse.Status, accrualResponse.Accrual); err != nil {
								// Log error but continue
								if lss.logger != nil {
									lss.logger.Errorw("Error updating order status and accrual", "error", err, "order", orderNumber)
								}
							}
						} else {
							// Update only status (for INVALID or PROCESSED without accrual)
							if err := lss.UpdateOrderStatus(pollCtx, orderNumber, accrualResponse.Status); err != nil {
								// Log error but continue
								if lss.logger != nil {
									lss.logger.Errorw("Error updating order status", "error", err, "order", orderNumber)
								}
							}
						}

						// Log final status
						if lss.logger != nil {
							lss.logger.Infow("Order reached final status", "order", orderNumber, "status", accrualResponse.Status)
						}

						return // Exit the polling loop as we reached a final state
					}

					// For non-final statuses (NEW, PROCESSING), continue polling
					continue

				case http.StatusNoContent:
					// Order not registered yet in accrual system, continue polling
					resp.Body.Close()
					if lss.logger != nil {
						lss.logger.Debugw("Order not registered in accrual system yet", "order", orderNumber)
					}
					continue

				case http.StatusTooManyRequests:
					// Rate limited, respect Retry-After header or wait 30 seconds
					retryAfter := 30 * time.Second // Default fallback
					if retryAfterHeader := resp.Header.Get("Retry-After"); retryAfterHeader != "" {
						if seconds, err := strconv.Atoi(retryAfterHeader); err == nil {
							retryAfter = time.Duration(seconds) * time.Second
						}
					}

					if lss.logger != nil {
						lss.logger.Warnw("Rate limited by accrual system", "order", orderNumber, "retryAfter", retryAfter.String())
					}

					resp.Body.Close()
					time.Sleep(retryAfter)
					continue

				default:
					// Unexpected response, log and continue polling
					if lss.logger != nil {
						lss.logger.Errorw("Unexpected response from accrual system", "statusCode", resp.StatusCode, "order", orderNumber)
					}
					resp.Body.Close()
					continue
				}
			}
		}
	}()
}

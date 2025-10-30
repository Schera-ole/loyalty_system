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

func (lss *LoyaltySystemService) PollOrderStatus(ctx context.Context, orderNumber string, accrualAddress string) {
	go func() {
		timeout := 2 * time.Minute
		var cancel context.CancelFunc
		pollCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-pollCtx.Done():
				return
			case <-ticker.C:
				accrualURL := accrualAddress + "/api/orders/" + orderNumber
				resp, err := http.Get(accrualURL)
				if err != nil {
					if lss.logger != nil {
						lss.logger.Errorw("Error making request to accrual system", "error", err, "order", orderNumber)
					}
					continue
				}

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

					if lss.logger != nil {
						lss.logger.Infow("Received accrual response", "order", orderNumber, "status", accrualResponse.Status)
					}

					// Check if status is final
					if accrualResponse.Status == "PROCESSED" || accrualResponse.Status == "INVALID" {
						if accrualResponse.Status == "PROCESSED" && accrualResponse.Accrual != nil {
							// Update both status and accrual
							if err := lss.UpdateOrderStatusAndAccrual(ctx, orderNumber, accrualResponse.Status, accrualResponse.Accrual); err != nil {
								if lss.logger != nil {
									lss.logger.Errorw("Error updating order status and accrual", "error", err, "order", orderNumber)
								}
							}
						} else {
							// Update only status (for INVALID or PROCESSED)
							if err := lss.UpdateOrderStatus(ctx, orderNumber, accrualResponse.Status); err != nil {
								if lss.logger != nil {
									lss.logger.Errorw("Error updating order status", "error", err, "order", orderNumber)
								}
							}
						}

						if lss.logger != nil {
							lss.logger.Infow("Order reached final status", "order", orderNumber, "status", accrualResponse.Status)
						}

						return
					}

					// For non-final statuses continue polling
					continue

				case http.StatusNoContent:
					resp.Body.Close()
					if lss.logger != nil {
						lss.logger.Debugw("Order not registered in accrual system yet", "order", orderNumber)
					}
					continue

				case http.StatusTooManyRequests:
					// Rate limited
					retryAfter := 30 * time.Second
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
					// Unexpected response, log and continue
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

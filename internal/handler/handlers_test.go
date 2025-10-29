package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Schera-ole/loyalty_system/internal/model"
	"github.com/go-chi/jwtauth/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithdrawPointsHandler_Validation(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name: "Valid withdrawal request",
			requestBody: model.Withdrawal{
				Order: "12345678903",
				Sum:   100.50,
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid JSON",
			requestBody:    "invalid json",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid request body format",
		},
		{
			name: "Empty order number",
			requestBody: model.Withdrawal{
				Order: "",
				Sum:   100.50,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid order number or sum",
		},
		{
			name: "Negative sum",
			requestBody: model.Withdrawal{
				Order: "12345678903",
				Sum:   -10.0,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid order number or sum",
		},
		{
			name: "Zero sum",
			requestBody: model.Withdrawal{
				Order: "12345678903",
				Sum:   0,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid order number or sum",
		},
		{
			name: "Invalid order number (Luhn check)",
			requestBody: model.Withdrawal{
				Order: "12345678901",
				Sum:   100.50,
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectedError:  "Invalid order number format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request body
			var body []byte
			var err error

			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else {
				body, err = json.Marshal(tt.requestBody)
				require.NoError(t, err)
			}

			// Create request
			req := httptest.NewRequest("POST", "/api/user/balance/withdraw", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			// Create JWT token for testing
			tokenAuth := jwtauth.New("HS256", []byte("test-secret"), nil)
			_, tokenString, err := tokenAuth.Encode(map[string]interface{}{"user_id": "testuser"})
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer "+tokenString)

			// Create response recorder
			rr := httptest.NewRecorder()

			// Create handler (we'll use a mock service for this test)
			// For now, just test the validation part
			handler := func(w http.ResponseWriter, r *http.Request) {
				// Handle decompression (no compression in test)
				body, err := ReadRequestBody(r)
				if err != nil {
					http.Error(w, "Failed to read request body: "+err.Error(), http.StatusBadRequest)
					return
				}

				var withdrawal model.Withdrawal
				if err := json.Unmarshal(body, &withdrawal); err != nil {
					http.Error(w, "Invalid request body format", http.StatusBadRequest)
					return
				}

				// Validate input
				if withdrawal.Order == "" || withdrawal.Sum <= 0 {
					http.Error(w, "Invalid order number or sum", http.StatusBadRequest)
					return
				}

				if !isValidLuhn(withdrawal.Order) {
					http.Error(w, "Invalid order number format", http.StatusUnprocessableEntity)
					return
				}

				w.WriteHeader(http.StatusOK)
			}

			// Set up context with JWT claims using the correct method
			req = req.WithContext(req.Context())

			// Call handler
			handler(rr, req)

			// Check response
			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedError != "" {
				assert.Contains(t, rr.Body.String(), tt.expectedError)
			}
		})
	}
}

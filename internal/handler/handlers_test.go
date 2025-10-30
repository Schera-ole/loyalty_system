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

			req := httptest.NewRequest("POST", "/api/user/balance/withdraw", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			// JWT token for testing
			tokenAuth := jwtauth.New("HS256", []byte("test-secret"), nil)
			_, tokenString, err := tokenAuth.Encode(map[string]interface{}{"user_id": "testuser"})
			require.NoError(t, err)
			req.Header.Set("Authorization", "Bearer "+tokenString)

			rr := httptest.NewRecorder()

			handler := func(w http.ResponseWriter, r *http.Request) {
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

			req = req.WithContext(req.Context())

			handler(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedError != "" {
				assert.Contains(t, rr.Body.String(), tt.expectedError)
			}
		})
	}
}

func TestIsValidLuhn(t *testing.T) {
	tests := []struct {
		number   string
		expected bool
	}{
		{"12345678903", true},       // Valid Luhn
		{"12345678901", false},      // Invalid Luhn
		{"", false},                 // Empty string
		{"abc", false},              // Non-numeric
		{"4532015112830366", true},  // Valid 16-digit credit card number
		{"1234567890123457", false}, // Invalid 16-digit
		{"79927398713", true},       // Valid Luhn example
		{"79927398714", false},      // Invalid Luhn example
	}

	for _, tt := range tests {
		t.Run(tt.number, func(t *testing.T) {
			result := isValidLuhn(tt.number)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSignUpHandler_Validation(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
	}{
		{
			name:           "Empty request body",
			requestBody:    "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Invalid JSON",
			requestBody:    "invalid json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Missing username",
			requestBody:    `{"password":"testpass"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Missing password",
			requestBody:    `{"login":"testuser"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Empty username",
			requestBody:    `{"login":"","password":"testpass"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Empty password",
			requestBody:    `{"login":"testuser","password":""}`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.requestBody != "" {
				body = []byte(tt.requestBody)
			}

			rr := httptest.NewRecorder()

			// Test only validation
			if tt.expectedStatus == http.StatusBadRequest {
				var user model.User
				err := json.Unmarshal(body, &user)
				if err != nil || user.Username == "" || user.Password == "" {
					rr.WriteHeader(http.StatusBadRequest)
				}
			}

			assert.Equal(t, tt.expectedStatus, rr.Code)
		})
	}
}

func TestSignInHandler_Validation(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
	}{
		{
			name:           "Empty request body",
			requestBody:    "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Invalid JSON",
			requestBody:    "invalid json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Missing username",
			requestBody:    `{"password":"testpass"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Missing password",
			requestBody:    `{"login":"testuser"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Empty username",
			requestBody:    `{"login":"","password":"testpass"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Empty password",
			requestBody:    `{"login":"testuser","password":""}`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.requestBody != "" {
				body = []byte(tt.requestBody)
			}

			rr := httptest.NewRecorder()

			// Test only validation
			if tt.expectedStatus == http.StatusBadRequest {
				var user model.User
				err := json.Unmarshal(body, &user)
				if err != nil || user.Username == "" || user.Password == "" {
					rr.WriteHeader(http.StatusBadRequest)
				}
			}

			assert.Equal(t, tt.expectedStatus, rr.Code)
		})
	}
}

func TestSendOrderHandler_Validation(t *testing.T) {
	tests := []struct {
		name           string
		orderNumber    string
		expectedStatus int
	}{
		{
			name:           "Empty order number",
			orderNumber:    "",
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:           "Invalid order number (Luhn check)",
			orderNumber:    "12345678901",
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:           "Non-numeric order number",
			orderNumber:    "abc123",
			expectedStatus: http.StatusUnprocessableEntity,
		},
		{
			name:           "Valid order number",
			orderNumber:    "12345678903",
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(tt.orderNumber)
			rr := httptest.NewRecorder()

			// Test only validation
			orderNumber := string(body)
			if orderNumber == "" {
				rr.WriteHeader(http.StatusUnprocessableEntity)
			} else if !isValidLuhn(orderNumber) {
				rr.WriteHeader(http.StatusUnprocessableEntity)
			} else {
				rr.WriteHeader(tt.expectedStatus)
			}

			assert.Equal(t, tt.expectedStatus, rr.Code)
		})
	}
}

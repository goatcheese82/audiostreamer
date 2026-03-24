package api

import (
	"context"
	"crypto/subtle"
	"log"
	"net/http"
	"strings"

	"github.com/audiostreamer/internal/db"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const accountKey contextKey = "account"

// AccountFromContext retrieves the authenticated account from the request context.
func AccountFromContext(ctx context.Context) *db.Account {
	a, _ := ctx.Value(accountKey).(*db.Account)
	return a
}

// AuthMiddleware authenticates requests using a pre-shared secret.
//
// Device auth flow:
//   - ESP32 sends header: Authorization: Bearer <account-name>:<secret>
//   - Middleware looks up account by name, compares bcrypt hash
//   - On success, stores account in context and updates device registration
//
// The device ID is taken from the X-Device-ID header (ESP32 MAC address).
type AuthMiddleware struct {
	store *db.Store
}

func NewAuthMiddleware(store *db.Store) *AuthMiddleware {
	return &AuthMiddleware{store: store}
}

// RequireAccount enforces device authentication on a handler.
func (m *AuthMiddleware) RequireAccount(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account, err := m.authenticate(r)
		if err != nil {
			log.Printf("[auth] rejected: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}

		// Register/update device if header present
		if deviceID := r.Header.Get("X-Device-ID"); deviceID != "" {
			m.store.UpsertDevice(r.Context(), &db.Device{
				DeviceID:  deviceID,
				AccountID: account.ID,
			})
		}

		ctx := context.WithValue(r.Context(), accountKey, account)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin enforces that the authenticated account has admin privileges.
func (m *AuthMiddleware) RequireAdmin(next http.Handler) http.Handler {
	return m.RequireAccount(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account := AccountFromContext(r.Context())
		if account == nil || !account.IsAdmin {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":"admin required"}`))
			return
		}
		next.ServeHTTP(w, r)
	}))
}

// AdminTokenAuth is simpler auth for the admin UI using a static token from config.
// Falls through to account-based auth if the admin token doesn't match.
func (m *AuthMiddleware) AdminOrAccount(adminToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)

			// Try static admin token first
			if adminToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(adminToken)) == 1 {
				// Admin token matches — create a synthetic admin account for context
				ctx := context.WithValue(r.Context(), accountKey, &db.Account{
					ID:      "admin",
					Name:    "admin",
					IsAdmin: true,
				})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Fall through to account-based auth
			m.RequireAccount(next).ServeHTTP(w, r)
		})
	}
}

func (m *AuthMiddleware) authenticate(r *http.Request) (*db.Account, error) {
	token := extractToken(r)
	if token == "" {
		return nil, ErrNoToken
	}

	// Token format: "account-name:secret"
	parts := strings.SplitN(token, ":", 2)
	if len(parts) != 2 {
		return nil, ErrBadToken
	}

	accountName := parts[0]
	secret := parts[1]

	account, err := m.store.GetAccountByName(r.Context(), accountName)
	if err != nil {
		return nil, ErrBadCredentials
	}

	// Compare bcrypt hash
	if err := bcrypt.CompareHashAndPassword([]byte(account.Secret), []byte(secret)); err != nil {
		return nil, ErrBadCredentials
	}

	return account, nil
}

func extractToken(r *http.Request) string {
	// Check Authorization header
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	// Check query parameter (useful for simple ESP32 GET requests)
	if token := r.URL.Query().Get("token"); token != "" {
		return token
	}

	return ""
}

// Sentinel errors
type authError string

func (e authError) Error() string { return string(e) }

const (
	ErrNoToken        = authError("no authentication token provided")
	ErrBadToken       = authError("invalid token format, expected name:secret")
	ErrBadCredentials = authError("invalid account name or secret")
)

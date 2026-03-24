package api

import (
	"log"
	"net/http"

	"github.com/audiostreamer/internal/db"
	"golang.org/x/crypto/bcrypt"
)

type AccountsHandler struct {
	store *db.Store
}

func NewAccountsHandler(store *db.Store) *AccountsHandler {
	return &AccountsHandler{store: store}
}

// ListAccounts returns all accounts (secrets omitted)
// GET /api/accounts
func (h *AccountsHandler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.store.ListAccounts(r.Context())
	if err != nil {
		log.Printf("[accounts] error listing: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list accounts")
		return
	}
	if accounts == nil {
		accounts = []db.Account{}
	}
	writeJSON(w, http.StatusOK, accounts)
}

// CreateAccount creates a new account with a pre-shared secret
// POST /api/accounts
// Body: { "name": "kids-room", "secret": "some-password", "is_admin": false }
func (h *AccountsHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		Secret  string `json:"secret"`
		IsAdmin bool   `json:"is_admin"`
	}

	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Name == "" || req.Secret == "" {
		writeError(w, http.StatusBadRequest, "name and secret are required")
		return
	}

	// Hash the secret
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Secret), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[accounts] bcrypt error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to hash secret")
		return
	}

	account := db.Account{
		Name:    req.Name,
		Secret:  string(hash),
		IsAdmin: req.IsAdmin,
	}

	if err := h.store.CreateAccount(r.Context(), &account); err != nil {
		log.Printf("[accounts] error creating: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create account (name may already exist)")
		return
	}

	log.Printf("[accounts] created %q (admin=%v)", account.Name, account.IsAdmin)

	// Return without secret hash
	account.Secret = ""
	writeJSON(w, http.StatusCreated, account)
}

// DeleteAccount removes an account and all its associated data
// DELETE /api/accounts/{id}
func (h *AccountsHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteAccount(r.Context(), id); err != nil {
		log.Printf("[accounts] error deleting: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete account")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GrantAccess gives an account access to a book
// POST /api/access
// Body: { "account_id": "uuid", "book_id": "uuid" }
func (h *AccountsHandler) GrantAccess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID string `json:"account_id"`
		BookID    string `json:"book_id"`
	}

	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.AccountID == "" || req.BookID == "" {
		writeError(w, http.StatusBadRequest, "account_id and book_id are required")
		return
	}

	if err := h.store.GrantBookAccess(r.Context(), req.AccountID, req.BookID); err != nil {
		log.Printf("[access] error granting: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to grant access")
		return
	}

	log.Printf("[access] granted account %s access to book %s", req.AccountID, req.BookID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "granted"})
}

// RevokeAccess removes an account's access to a book
// DELETE /api/access
// Body: { "account_id": "uuid", "book_id": "uuid" }
func (h *AccountsHandler) RevokeAccess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID string `json:"account_id"`
		BookID    string `json:"book_id"`
	}

	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := h.store.RevokeBookAccess(r.Context(), req.AccountID, req.BookID); err != nil {
		log.Printf("[access] error revoking: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to revoke access")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// GrantAllAccess gives an account access to every book
// POST /api/access/all
// Body: { "account_id": "uuid" }
func (h *AccountsHandler) GrantAllAccess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID string `json:"account_id"`
	}

	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.AccountID == "" {
		writeError(w, http.StatusBadRequest, "account_id is required")
		return
	}

	count, err := h.store.GrantAllBooksAccess(r.Context(), req.AccountID)
	if err != nil {
		log.Printf("[access] error granting all: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to grant access")
		return
	}

	log.Printf("[access] granted account %s access to %d books", req.AccountID, count)
	writeJSON(w, http.StatusOK, map[string]any{"status": "granted", "count": count})
}

// ListBookAccess shows who has access to a specific book
// GET /api/access/book/{id}
func (h *AccountsHandler) ListBookAccess(w http.ResponseWriter, r *http.Request) {
	bookID := r.PathValue("id")
	access, err := h.store.ListBookAccessByBook(r.Context(), bookID)
	if err != nil {
		log.Printf("[access] error listing: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list access")
		return
	}
	if access == nil {
		access = []db.BookAccess{}
	}
	writeJSON(w, http.StatusOK, access)
}

// ListAccountBooks shows which books an account can access
// GET /api/access/account/{id}
func (h *AccountsHandler) ListAccountBooks(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("id")
	books, err := h.store.ListAccountBooks(r.Context(), accountID)
	if err != nil {
		log.Printf("[access] error listing account books: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list books")
		return
	}
	if books == nil {
		books = []db.Book{}
	}
	writeJSON(w, http.StatusOK, books)
}

package api

import (
	"log"
	"net/http"

	"github.com/audiostreamer/internal/db"
	"github.com/audiostreamer/internal/stream"
)

type BooksHandler struct {
	store    *db.Store
	basePath string
}

func NewBooksHandler(store *db.Store, basePath string) *BooksHandler {
	return &BooksHandler{store: store, basePath: basePath}
}

// ListBooks returns all books in the library
// GET /api/books
func (h *BooksHandler) ListBooks(w http.ResponseWriter, r *http.Request) {
	books, err := h.store.ListBooks(r.Context())
	if err != nil {
		log.Printf("[books] error listing: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list books")
		return
	}
	if books == nil {
		books = []db.Book{}
	}
	writeJSON(w, http.StatusOK, books)
}

// GetBook returns a single book by ID
// GET /api/books/{id}
func (h *BooksHandler) GetBook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	book, err := h.store.GetBook(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "book not found")
		return
	}
	writeJSON(w, http.StatusOK, book)
}

// CreateBook creates a new book manually
// POST /api/books
func (h *BooksHandler) CreateBook(w http.ResponseWriter, r *http.Request) {
	var book db.Book
	if err := readJSON(r, &book); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if book.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	if err := h.store.CreateBook(r.Context(), &book); err != nil {
		log.Printf("[books] error creating: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create book")
		return
	}

	writeJSON(w, http.StatusCreated, book)
}

// UpdateBook updates an existing book
// PUT /api/books/{id}
func (h *BooksHandler) UpdateBook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var book db.Book
	if err := readJSON(r, &book); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	book.ID = id

	if err := h.store.UpdateBook(r.Context(), &book); err != nil {
		log.Printf("[books] error updating: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update book")
		return
	}

	writeJSON(w, http.StatusOK, book)
}

// DeleteBook deletes a book
// DELETE /api/books/{id}
func (h *BooksHandler) DeleteBook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteBook(r.Context(), id); err != nil {
		log.Printf("[books] error deleting: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete book")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ScanBooks scans the audiobook directory and imports new books
// POST /api/books/scan
func (h *BooksHandler) ScanBooks(w http.ResponseWriter, r *http.Request) {
	scanned, err := stream.ScanDirectory(h.basePath)
	if err != nil {
		log.Printf("[books] scan error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to scan directory")
		return
	}

	var created, skipped int

	for _, s := range scanned {
		book := db.Book{
			Title:     s.Title,
			Author:    s.Author,
			FilePaths: s.FilePaths,
		}

		if err := h.store.CreateBook(r.Context(), &book); err != nil {
			// Likely duplicate — skip
			skipped++
			continue
		}
		created++
	}

	log.Printf("[books] scan complete: %d found, %d created, %d skipped", len(scanned), created, skipped)

	writeJSON(w, http.StatusOK, map[string]any{
		"found":   len(scanned),
		"created": created,
		"skipped": skipped,
	})
}

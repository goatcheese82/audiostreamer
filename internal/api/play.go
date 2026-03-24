package api

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/audiostreamer/internal/db"
	"github.com/audiostreamer/internal/stream"
	"github.com/jackc/pgx/v5"
)

type PlayHandler struct {
	store      *db.Store
	transcoder *stream.Transcoder
}

func NewPlayHandler(store *db.Store, transcoder *stream.Transcoder) *PlayHandler {
	return &PlayHandler{store: store, transcoder: transcoder}
}

// Play streams an audiobook associated with the given NFC tag UID.
// The ESP32 calls this endpoint when a tag is scanned.
//
// GET /api/play/{nfc_id}
//
// Query params:
//   - device: ESP32 device ID (MAC address) for per-device progress tracking
//   - pos:    override start position in seconds (skip resume logic)
//
// Response: audio/ogg stream (Transfer-Encoding: chunked)
func (h *PlayHandler) Play(w http.ResponseWriter, r *http.Request) {
	nfcID := r.PathValue("nfc_id")
	deviceID := r.URL.Query().Get("device")

	// Look up which book this tag maps to
	book, err := h.store.GetBookByTagUID(r.Context(), nfcID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Printf("[play] unknown tag: %s", nfcID)
			http.Error(w, "unknown tag", http.StatusNotFound)
			return
		}
		log.Printf("[play] error looking up tag %s: %v", nfcID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if len(book.FilePaths) == 0 {
		log.Printf("[play] book %s has no audio files", book.ID)
		http.Error(w, "book has no audio files", http.StatusNotFound)
		return
	}

	// Check book access for authenticated account
	account := AccountFromContext(r.Context())
	if account != nil && !account.IsAdmin {
		hasAccess, err := h.store.HasBookAccess(r.Context(), account.ID, book.ID)
		if err != nil || !hasAccess {
			log.Printf("[play] access denied: account %s (%s) → book %s", account.ID, account.Name, book.ID)
			writeError(w, http.StatusForbidden, "no access to this book")
			return
		}
	}

	// Determine start position
	var seekSec float64

	// Check for explicit position override
	if posStr := r.URL.Query().Get("pos"); posStr != "" {
		fmt.Sscanf(posStr, "%f", &seekSec)
	} else {
		// Try to resume from saved progress
		seekSec = h.getResumePosition(r, book.ID, deviceID)
	}

	log.Printf("[play] tag=%s book=%q seek=%.1fs files=%d",
		nfcID, book.Title, seekSec, len(book.FilePaths))

	// Start transcoding
	result, err := h.transcoder.StreamConcat(r.Context(), book.FilePaths, seekSec)
	if err != nil {
		log.Printf("[play] transcode error: %v", err)
		http.Error(w, "transcoding error", http.StatusInternalServerError)
		return
	}
	defer result.Close()

	// Stream the audio
	w.Header().Set("Content-Type", "audio/ogg")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Book-ID", book.ID)
	w.Header().Set("X-Book-Title", book.Title)
	w.Header().Set("X-Seek-Position", fmt.Sprintf("%.1f", seekSec))
	w.WriteHeader(http.StatusOK)

	// Flush headers immediately so the ESP32 can start buffering
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Copy audio data to response
	buf := make([]byte, 8192)
	for {
		n, err := result.Reader.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				log.Printf("[play] client disconnected: %v", writeErr)
				return
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("[play] read error: %v", err)
			}
			return
		}
	}
}

func (h *PlayHandler) getResumePosition(r *http.Request, bookID, deviceID string) float64 {
	var progress *db.PlaybackProgress
	var err error

	if deviceID != "" {
		progress, err = h.store.GetProgress(r.Context(), bookID, deviceID)
	} else {
		progress, err = h.store.GetProgressAnyDevice(r.Context(), bookID)
	}

	if err != nil {
		// No saved progress — start from beginning
		return 0
	}

	if progress.IsFinished {
		// Book was finished — start from beginning
		return 0
	}

	return progress.PositionSec
}

// GetBookInfo returns metadata about the book associated with a tag
//
// GET /api/book/{nfc_id}
func (h *PlayHandler) GetBookInfo(w http.ResponseWriter, r *http.Request) {
	nfcID := r.PathValue("nfc_id")
	deviceID := r.URL.Query().Get("device")

	book, err := h.store.GetBookByTagUID(r.Context(), nfcID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "unknown tag", http.StatusNotFound)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Check book access
	account := AccountFromContext(r.Context())
	if account != nil && !account.IsAdmin {
		hasAccess, err := h.store.HasBookAccess(r.Context(), account.ID, book.ID)
		if err != nil || !hasAccess {
			writeError(w, http.StatusForbidden, "no access to this book")
			return
		}
	}

	type response struct {
		Book        *db.Book `json:"book"`
		PositionSec float64  `json:"position_sec"`
		IsFinished  bool     `json:"is_finished"`
	}

	resp := response{Book: book}

	if deviceID != "" {
		if p, err := h.store.GetProgress(r.Context(), book.ID, deviceID); err == nil {
			resp.PositionSec = p.PositionSec
			resp.IsFinished = p.IsFinished
		}
	} else {
		if p, err := h.store.GetProgressAnyDevice(r.Context(), book.ID); err == nil {
			resp.PositionSec = p.PositionSec
			resp.IsFinished = p.IsFinished
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

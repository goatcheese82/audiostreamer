package api

import (
	"log"
	"net/http"

	"github.com/audiostreamer/internal/db"
)

type ProgressHandler struct {
	store *db.Store
}

func NewProgressHandler(store *db.Store) *ProgressHandler {
	return &ProgressHandler{store: store}
}

// UpdateProgress saves the current playback position.
// Called periodically by the ESP32 during playback.
//
// POST /api/progress/{nfc_id}
// Body: { "device": "AA:BB:CC:DD:EE:FF", "position_sec": 3742.5, "is_finished": false }
func (h *ProgressHandler) UpdateProgress(w http.ResponseWriter, r *http.Request) {
	nfcID := r.PathValue("nfc_id")
	account := AccountFromContext(r.Context())

	var req struct {
		DeviceID    string  `json:"device"`
		PositionSec float64 `json:"position_sec"`
		IsFinished  bool    `json:"is_finished"`
	}

	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.DeviceID == "" {
		writeError(w, http.StatusBadRequest, "device is required")
		return
	}

	// Look up book from tag
	book, err := h.store.GetBookByTagUID(r.Context(), nfcID)
	if err != nil {
		writeError(w, http.StatusNotFound, "unknown tag")
		return
	}

	// Determine account ID
	accountID := ""
	if account != nil {
		accountID = account.ID
	}

	// Save progress
	err = h.store.UpsertProgress(r.Context(), &db.PlaybackProgress{
		BookID:      book.ID,
		AccountID:   accountID,
		DeviceID:    req.DeviceID,
		PositionSec: req.PositionSec,
		IsFinished:  req.IsFinished,
	})
	if err != nil {
		log.Printf("[progress] error saving: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to save progress")
		return
	}

	log.Printf("[progress] tag=%s book=%q pos=%.1fs finished=%v",
		nfcID, book.Title, req.PositionSec, req.IsFinished)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// StopPlayback marks the current position and signals playback stopped.
//
// POST /api/stop/{nfc_id}
// Body: { "device": "AA:BB:CC:DD:EE:FF", "position_sec": 3742.5 }
func (h *ProgressHandler) StopPlayback(w http.ResponseWriter, r *http.Request) {
	nfcID := r.PathValue("nfc_id")
	account := AccountFromContext(r.Context())

	var req struct {
		DeviceID    string  `json:"device"`
		PositionSec float64 `json:"position_sec"`
	}

	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.DeviceID == "" {
		writeError(w, http.StatusBadRequest, "device is required")
		return
	}

	book, err := h.store.GetBookByTagUID(r.Context(), nfcID)
	if err != nil {
		writeError(w, http.StatusNotFound, "unknown tag")
		return
	}

	accountID := ""
	if account != nil {
		accountID = account.ID
	}

	err = h.store.UpsertProgress(r.Context(), &db.PlaybackProgress{
		BookID:      book.ID,
		AccountID:   accountID,
		DeviceID:    req.DeviceID,
		PositionSec: req.PositionSec,
		IsFinished:  false,
	})
	if err != nil {
		log.Printf("[stop] error saving: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to save progress")
		return
	}

	log.Printf("[stop] tag=%s book=%q pos=%.1fs", nfcID, book.Title, req.PositionSec)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

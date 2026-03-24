package api

import (
	"log"
	"net/http"

	"github.com/audiostreamer/internal/db"
)

type DevicesHandler struct {
	store *db.Store
}

func NewDevicesHandler(store *db.Store) *DevicesHandler {
	return &DevicesHandler{store: store}
}

// ListDevices returns all registered ESP32 devices
// GET /api/devices
func (h *DevicesHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := h.store.ListDevices(r.Context())
	if err != nil {
		log.Printf("[devices] error listing: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list devices")
		return
	}
	if devices == nil {
		devices = []db.Device{}
	}
	writeJSON(w, http.StatusOK, devices)
}

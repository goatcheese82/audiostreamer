package api

import (
	"log"
	"net/http"

	"github.com/audiostreamer/internal/db"
)

type TagsHandler struct {
	store *db.Store
}

func NewTagsHandler(store *db.Store) *TagsHandler {
	return &TagsHandler{store: store}
}

// ListTags returns all NFC tag mappings
// GET /api/tags
func (h *TagsHandler) ListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.store.ListTags(r.Context())
	if err != nil {
		log.Printf("[tags] error listing: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list tags")
		return
	}
	if tags == nil {
		tags = []db.NFCTag{}
	}
	writeJSON(w, http.StatusOK, tags)
}

// CreateTag assigns an NFC tag to a book
// POST /api/tags
// Body: { "tag_uid": "04A32B1C5E8000", "book_id": "uuid", "label": "Blue tag" }
func (h *TagsHandler) CreateTag(w http.ResponseWriter, r *http.Request) {
	var tag db.NFCTag
	if err := readJSON(r, &tag); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if tag.TagUID == "" || tag.BookID == "" {
		writeError(w, http.StatusBadRequest, "tag_uid and book_id are required")
		return
	}

	if err := h.store.CreateTag(r.Context(), &tag); err != nil {
		log.Printf("[tags] error creating: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create tag")
		return
	}

	log.Printf("[tags] mapped %s → book %s (%s)", tag.TagUID, tag.BookID, tag.Label)
	writeJSON(w, http.StatusCreated, tag)
}

// DeleteTag removes an NFC tag mapping
// DELETE /api/tags/{tag_uid}
func (h *TagsHandler) DeleteTag(w http.ResponseWriter, r *http.Request) {
	tagUID := r.PathValue("tag_uid")
	if err := h.store.DeleteTag(r.Context(), tagUID); err != nil {
		log.Printf("[tags] error deleting: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete tag")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GetTag returns a single tag mapping
// GET /api/tags/{tag_uid}
func (h *TagsHandler) GetTag(w http.ResponseWriter, r *http.Request) {
	tagUID := r.PathValue("tag_uid")
	tag, err := h.store.GetTag(r.Context(), tagUID)
	if err != nil {
		writeError(w, http.StatusNotFound, "tag not found")
		return
	}
	writeJSON(w, http.StatusOK, tag)
}

// RegisterTag is called by the ESP32 when it reads a tag.
// If the tag is unknown, it's stored as pending (no book assigned).
// This supports the "scan mode" workflow from the admin UI.
//
// POST /api/tags/register
// Body: { "tag_uid": "04A32B1C5E8000", "device": "AA:BB:CC:DD:EE:FF" }
func (h *TagsHandler) RegisterTag(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TagUID   string `json:"tag_uid"`
		DeviceID string `json:"device"`
	}

	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.TagUID == "" {
		writeError(w, http.StatusBadRequest, "tag_uid is required")
		return
	}

	// Check if tag already exists
	existing, err := h.store.GetTag(r.Context(), req.TagUID)
	if err == nil {
		// Tag exists — return its mapping
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "known",
			"tag":    existing,
		})
		return
	}

	// Tag is new/unassigned
	log.Printf("[tags] new tag scanned: %s from device %s", req.TagUID, req.DeviceID)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "unknown",
		"tag_uid": req.TagUID,
		"message": "tag not assigned to any book",
	})
}

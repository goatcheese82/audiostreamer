package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/audiostreamer/internal/db"
)

type ImportHandler struct {
	store    *db.Store
	absURL   string
	absToken string
	basePath string
}

func NewImportHandler(store *db.Store, absURL, absToken, basePath string) *ImportHandler {
	return &ImportHandler{
		store:    store,
		absURL:   absURL,
		absToken: absToken,
		basePath: basePath,
	}
}

// ImportFromABS imports audiobook metadata from Audiobookshelf
// POST /api/books/import
func (h *ImportHandler) ImportFromABS(w http.ResponseWriter, r *http.Request) {
	if h.absURL == "" || h.absToken == "" {
		writeError(w, http.StatusBadRequest, "Audiobookshelf URL and token not configured")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	// Get libraries
	libraries, err := h.absGetLibraries(ctx)
	if err != nil {
		log.Printf("[import] error getting libraries: %v", err)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get libraries: %v", err))
		return
	}

	var imported, updated, skipped int

	for _, lib := range libraries {
		if lib.MediaType != "book" {
			continue
		}

		items, err := h.absGetLibraryItems(ctx, lib.ID)
		if err != nil {
			log.Printf("[import] error getting items for library %s: %v", lib.ID, err)
			continue
		}

		for _, item := range items {
			book := h.absItemToBook(item)
			if len(book.FilePaths) == 0 {
				skipped++
				continue
			}

			if err := h.store.UpsertBookByABSID(ctx, &book); err != nil {
				log.Printf("[import] error upserting book %s: %v", book.Title, err)
				skipped++
				continue
			}
			imported++
		}
	}

	log.Printf("[import] complete: %d imported, %d updated, %d skipped", imported, updated, skipped)

	writeJSON(w, http.StatusOK, map[string]any{
		"imported": imported,
		"updated":  updated,
		"skipped":  skipped,
	})
}

// ABS API types (minimal, only what we need)

type absLibrary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	MediaType string `json:"mediaType"`
}

type absLibraryItem struct {
	ID    string `json:"id"`
	Path  string `json:"path"`
	Media struct {
		Metadata struct {
			Title       string   `json:"title"`
			AuthorName  string   `json:"authorName"`
			Narrator    string   `json:"narratorName"`
			Description string   `json:"description"`
			Genres      []string `json:"genres"`
		} `json:"metadata"`
		Duration   float64 `json:"duration"`
		AudioFiles []struct {
			INO      string  `json:"ino"`
			Metadata struct {
				Filename string `json:"filename"`
				Path     string `json:"path"`
			} `json:"metadata"`
			Duration float64 `json:"duration"`
		} `json:"audioFiles"`
	} `json:"media"`
}

func (h *ImportHandler) absRequest(ctx context.Context, path string) ([]byte, error) {
	url := strings.TrimRight(h.absURL, "/") + path

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+h.absToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ABS request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ABS returned %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

func (h *ImportHandler) absGetLibraries(ctx context.Context) ([]absLibrary, error) {
	data, err := h.absRequest(ctx, "/api/libraries")
	if err != nil {
		return nil, err
	}

	var resp struct {
		Libraries []absLibrary `json:"libraries"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing libraries: %w", err)
	}
	return resp.Libraries, nil
}

func (h *ImportHandler) absGetLibraryItems(ctx context.Context, libraryID string) ([]absLibraryItem, error) {
	// Fetch all items (ABS paginates, so we request a large limit)
	data, err := h.absRequest(ctx, fmt.Sprintf("/api/libraries/%s/items?limit=10000&expanded=1", libraryID))
	if err != nil {
		return nil, err
	}

	var resp struct {
		Results []absLibraryItem `json:"results"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing library items: %w", err)
	}
	return resp.Results, nil
}

func (h *ImportHandler) absItemToBook(item absLibraryItem) db.Book {
	var filePaths []string
	for _, af := range item.Media.AudioFiles {
		// Map ABS file paths to our mount paths
		// ABS stores paths relative to its library root
		// We need to translate to /mnt/tower/... paths
		absPath := af.Metadata.Path
		if absPath != "" {
			// Try to map the path — this depends on how ABS and our mounts align
			localPath := h.mapABSPath(absPath)
			filePaths = append(filePaths, localPath)
		}
	}

	return db.Book{
		Title:       item.Media.Metadata.Title,
		Author:      item.Media.Metadata.AuthorName,
		Narrator:    item.Media.Metadata.Narrator,
		Description: item.Media.Metadata.Description,
		DurationSec: int(item.Media.Duration),
		FilePaths:   filePaths,
		ABSID:       item.ID,
	}
}

// mapABSPath converts an Audiobookshelf file path to a local mount path.
// This is a best-effort mapping that assumes the ABS library root and
// our mount point share the same subdirectory structure.
//
// Example:
//   ABS path:   /audiobooks/Author/Title/chapter01.mp3
//   Our mount:  /mnt/tower/audiobooks/Author/Title/chapter01.mp3
func (h *ImportHandler) mapABSPath(absPath string) string {
	// If the path already starts with our base path, use it as-is
	if strings.HasPrefix(absPath, h.basePath) {
		return absPath
	}

	// Otherwise, take just the filename parts and append to our base path
	// Strip common prefixes like /audiobooks
	name := filepath.Base(absPath)
	dir := filepath.Dir(absPath)
	parentDir := filepath.Base(dir)

	// Try: basePath / parentDir / name
	return filepath.Join(h.basePath, parentDir, name)
}

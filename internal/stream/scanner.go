package stream

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// AudioExtensions lists recognized audio file extensions
var AudioExtensions = map[string]bool{
	".mp3":  true,
	".m4b":  true,
	".m4a":  true,
	".ogg":  true,
	".opus": true,
	".flac": true,
	".wma":  true,
}

// ScannedBook represents an audiobook found on disk
type ScannedBook struct {
	Title     string
	Author    string
	DirPath   string
	FilePaths []string
}

// ScanDirectory scans a base directory for audiobooks.
// It looks for directories containing audio files, treating each directory
// as a separate book. It also handles flat directories with single-file books.
//
// Expected structures:
//   /audiobooks/Author Name/Book Title/chapter01.mp3
//   /audiobooks/Author Name/Book Title.m4b
//   /audiobooks/Book Title/chapter01.mp3
//   /audiobooks/Book Title.m4b
func ScanDirectory(basePath string) ([]ScannedBook, error) {
	var books []ScannedBook

	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil, fmt.Errorf("reading base directory %s: %w", basePath, err)
	}

	for _, entry := range entries {
		entryPath := filepath.Join(basePath, entry.Name())

		if !entry.IsDir() {
			// Single file in base directory
			if isAudioFile(entry.Name()) {
				books = append(books, ScannedBook{
					Title:     stripExtension(entry.Name()),
					Author:    "",
					DirPath:   basePath,
					FilePaths: []string{entryPath},
				})
			}
			continue
		}

		// It's a directory — could be author/title or just title
		found := scanBookDir(entryPath, entry.Name(), "")
		if len(found) > 0 {
			books = append(books, found...)
			continue
		}

		// Check subdirectories (author/title structure)
		subEntries, err := os.ReadDir(entryPath)
		if err != nil {
			log.Printf("[scan] warning: cannot read %s: %v", entryPath, err)
			continue
		}

		authorName := entry.Name()
		for _, sub := range subEntries {
			if !sub.IsDir() {
				// Single file under author directory
				subPath := filepath.Join(entryPath, sub.Name())
				if isAudioFile(sub.Name()) {
					books = append(books, ScannedBook{
						Title:     stripExtension(sub.Name()),
						Author:    authorName,
						DirPath:   entryPath,
						FilePaths: []string{subPath},
					})
				}
				continue
			}

			subPath := filepath.Join(entryPath, sub.Name())
			found := scanBookDir(subPath, sub.Name(), authorName)
			books = append(books, found...)
		}
	}

	log.Printf("[scan] found %d books in %s", len(books), basePath)
	return books, nil
}

// scanBookDir looks for audio files in a directory and returns a ScannedBook
// if any are found.
func scanBookDir(dirPath, title, author string) []ScannedBook {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil
	}

	var audioPaths []string
	for _, f := range files {
		if !f.IsDir() && isAudioFile(f.Name()) {
			audioPaths = append(audioPaths, filepath.Join(dirPath, f.Name()))
		}
	}

	if len(audioPaths) == 0 {
		return nil
	}

	// Sort files to maintain chapter order
	sort.Strings(audioPaths)

	return []ScannedBook{{
		Title:     title,
		Author:    author,
		DirPath:   dirPath,
		FilePaths: audioPaths,
	}}
}

func isAudioFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return AudioExtensions[ext]
}

func stripExtension(name string) string {
	ext := filepath.Ext(name)
	return strings.TrimSuffix(name, ext)
}

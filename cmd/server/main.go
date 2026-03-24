package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/audiostreamer/internal/api"
	"github.com/audiostreamer/internal/config"
	"github.com/audiostreamer/internal/db"
	"github.com/audiostreamer/internal/stream"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Database
	store, err := db.NewStore(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer store.Close()

	if err := store.RunMigrations(ctx); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("database migrations complete")

	// Transcoder
	transcoder := stream.NewTranscoder(cfg.FFmpegPath, cfg.OpusBitrate, cfg.SampleRate)

	// Auth middleware
	auth := api.NewAuthMiddleware(store)

	// Handlers
	playHandler := api.NewPlayHandler(store, transcoder)
	progressHandler := api.NewProgressHandler(store)
	booksHandler := api.NewBooksHandler(store, cfg.AudiobookBasePath)
	tagsHandler := api.NewTagsHandler(store)
	devicesHandler := api.NewDevicesHandler(store)
	accountsHandler := api.NewAccountsHandler(store)
	importHandler := api.NewImportHandler(store, cfg.ABSUrl, cfg.ABSToken, cfg.AudiobookBasePath)

	// Router (Go 1.22+ pattern matching)
	mux := http.NewServeMux()

	// Health check — no auth
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// --- ESP32 device endpoints (require account auth) ---
	mux.Handle("GET /api/play/{nfc_id}", auth.RequireAccount(http.HandlerFunc(playHandler.Play)))
	mux.Handle("GET /api/book/{nfc_id}", auth.RequireAccount(http.HandlerFunc(playHandler.GetBookInfo)))
	mux.Handle("POST /api/progress/{nfc_id}", auth.RequireAccount(http.HandlerFunc(progressHandler.UpdateProgress)))
	mux.Handle("POST /api/stop/{nfc_id}", auth.RequireAccount(http.HandlerFunc(progressHandler.StopPlayback)))
	mux.Handle("POST /api/tags/register", auth.RequireAccount(http.HandlerFunc(tagsHandler.RegisterTag)))

	// --- Admin endpoints (require admin token or admin account) ---
	adminAuth := auth.AdminOrAccount(cfg.AdminToken)

	// Books (global library, admin-managed)
	mux.Handle("POST /api/books/scan", adminAuth(http.HandlerFunc(booksHandler.ScanBooks)))
	mux.Handle("POST /api/books/import", adminAuth(http.HandlerFunc(importHandler.ImportFromABS)))
	mux.Handle("GET /api/books", adminAuth(http.HandlerFunc(booksHandler.ListBooks)))
	mux.Handle("POST /api/books", adminAuth(http.HandlerFunc(booksHandler.CreateBook)))
	mux.Handle("GET /api/books/{id}", adminAuth(http.HandlerFunc(booksHandler.GetBook)))
	mux.Handle("PUT /api/books/{id}", adminAuth(http.HandlerFunc(booksHandler.UpdateBook)))
	mux.Handle("DELETE /api/books/{id}", adminAuth(http.HandlerFunc(booksHandler.DeleteBook)))

	// Tags (admin-managed)
	mux.Handle("GET /api/tags", adminAuth(http.HandlerFunc(tagsHandler.ListTags)))
	mux.Handle("POST /api/tags", adminAuth(http.HandlerFunc(tagsHandler.CreateTag)))
	mux.Handle("GET /api/tags/{tag_uid}", adminAuth(http.HandlerFunc(tagsHandler.GetTag)))
	mux.Handle("DELETE /api/tags/{tag_uid}", adminAuth(http.HandlerFunc(tagsHandler.DeleteTag)))

	// Devices (admin-managed)
	mux.Handle("GET /api/devices", adminAuth(http.HandlerFunc(devicesHandler.ListDevices)))

	// Accounts (admin-managed)
	mux.Handle("GET /api/accounts", adminAuth(http.HandlerFunc(accountsHandler.ListAccounts)))
	mux.Handle("POST /api/accounts", adminAuth(http.HandlerFunc(accountsHandler.CreateAccount)))
	mux.Handle("DELETE /api/accounts/{id}", adminAuth(http.HandlerFunc(accountsHandler.DeleteAccount)))

	// Book access grants (admin-managed)
	mux.Handle("POST /api/access", adminAuth(http.HandlerFunc(accountsHandler.GrantAccess)))
	mux.Handle("DELETE /api/access", adminAuth(http.HandlerFunc(accountsHandler.RevokeAccess)))
	mux.Handle("POST /api/access/all", adminAuth(http.HandlerFunc(accountsHandler.GrantAllAccess)))
	mux.Handle("GET /api/access/book/{id}", adminAuth(http.HandlerFunc(accountsHandler.ListBookAccess)))
	mux.Handle("GET /api/access/account/{id}", adminAuth(http.HandlerFunc(accountsHandler.ListAccountBooks)))

	// Apply middleware stack
	handler := withRecovery(withLogging(withCORS(mux)))

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 300 * time.Second, // long timeout for audio streams
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		log.Println("shutting down server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	log.Printf("audiostreamer server starting on %s", addr)
	log.Printf("audiobook path: %s", cfg.AudiobookBasePath)
	if cfg.AdminToken != "" {
		log.Println("admin token: configured")
	} else {
		log.Println("WARNING: no ADMIN_TOKEN set — admin endpoints require account-based auth")
	}

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}

	log.Println("server stopped")
}

// --- Middleware ---

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}

		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Authorization, X-Device-ID")
		w.Header().Set("Access-Control-Expose-Headers", "X-Book-ID, X-Book-Title, X-Seek-Position")
		w.Header().Set("Access-Control-Max-Age", "300")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)

		// Skip logging for noisy progress updates
		if strings.HasPrefix(r.URL.Path, "/api/progress") {
			return
		}

		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond))
	})
}

func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

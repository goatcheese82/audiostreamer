package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) RunMigrations(ctx context.Context) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
			id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name            TEXT NOT NULL,
			secret          TEXT NOT NULL,
			is_admin        BOOLEAN NOT NULL DEFAULT false,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_name ON accounts(name)`,
		`CREATE TABLE IF NOT EXISTS books (
			id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			title           TEXT NOT NULL,
			author          TEXT NOT NULL DEFAULT '',
			narrator        TEXT NOT NULL DEFAULT '',
			description     TEXT NOT NULL DEFAULT '',
			cover_path      TEXT NOT NULL DEFAULT '',
			duration_sec    INTEGER NOT NULL DEFAULT 0,
			file_paths      TEXT[] NOT NULL DEFAULT '{}',
			abs_id          TEXT NOT NULL DEFAULT '',
			created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS book_access (
			account_id      UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
			book_id         UUID NOT NULL REFERENCES books(id) ON DELETE CASCADE,
			granted_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
			PRIMARY KEY (account_id, book_id)
		)`,
		`CREATE TABLE IF NOT EXISTS nfc_tags (
			tag_uid         TEXT PRIMARY KEY,
			book_id         UUID NOT NULL REFERENCES books(id) ON DELETE CASCADE,
			account_id      UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
			label           TEXT NOT NULL DEFAULT '',
			created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS playback_progress (
			id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			book_id         UUID NOT NULL REFERENCES books(id) ON DELETE CASCADE,
			account_id      UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
			device_id       TEXT NOT NULL,
			position_sec    REAL NOT NULL DEFAULT 0,
			is_finished     BOOLEAN NOT NULL DEFAULT false,
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(book_id, account_id, device_id)
		)`,
		`CREATE TABLE IF NOT EXISTS devices (
			device_id       TEXT PRIMARY KEY,
			account_id      UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
			name            TEXT NOT NULL DEFAULT '',
			last_seen       TIMESTAMPTZ,
			firmware_ver    TEXT NOT NULL DEFAULT '',
			created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_book_access_account ON book_access(account_id)`,
		`CREATE INDEX IF NOT EXISTS idx_book_access_book ON book_access(book_id)`,
		`CREATE INDEX IF NOT EXISTS idx_nfc_tags_book_id ON nfc_tags(book_id)`,
		`CREATE INDEX IF NOT EXISTS idx_nfc_tags_account_id ON nfc_tags(account_id)`,
		`CREATE INDEX IF NOT EXISTS idx_playback_progress_lookup ON playback_progress(book_id, account_id, device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_account_id ON devices(account_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_books_abs_id ON books(abs_id) WHERE abs_id != ''`,
	}

	for _, m := range migrations {
		if _, err := s.pool.Exec(ctx, m); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, m)
		}
	}

	return nil
}

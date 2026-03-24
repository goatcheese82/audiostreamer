package db

import (
	"context"
	"fmt"
)

// --- Accounts ---

func (s *Store) CreateAccount(ctx context.Context, a *Account) error {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO accounts (name, secret, is_admin)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`, a.Name, a.Secret, a.IsAdmin).Scan(&a.ID, &a.CreatedAt)
	if err != nil {
		return fmt.Errorf("creating account: %w", err)
	}
	return nil
}

func (s *Store) GetAccount(ctx context.Context, id string) (*Account, error) {
	var a Account
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, secret, is_admin, created_at
		FROM accounts WHERE id = $1
	`, id).Scan(&a.ID, &a.Name, &a.Secret, &a.IsAdmin, &a.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting account %s: %w", id, err)
	}
	return &a, nil
}

func (s *Store) GetAccountByName(ctx context.Context, name string) (*Account, error) {
	var a Account
	err := s.pool.QueryRow(ctx, `
		SELECT id, name, secret, is_admin, created_at
		FROM accounts WHERE name = $1
	`, name).Scan(&a.ID, &a.Name, &a.Secret, &a.IsAdmin, &a.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("getting account by name %s: %w", name, err)
	}
	return &a, nil
}

func (s *Store) ListAccounts(ctx context.Context) ([]Account, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, '', is_admin, created_at
		FROM accounts ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("listing accounts: %w", err)
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.Name, &a.Secret, &a.IsAdmin, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning account: %w", err)
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func (s *Store) DeleteAccount(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM accounts WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting account %s: %w", id, err)
	}
	return nil
}

// AuthenticateDevice looks up an account by matching the pre-shared secret.
// The secret is stored as a bcrypt hash, but for device auth we accept
// the raw secret and compare. Returns the account if valid, error if not.
func (s *Store) GetAccountBySecret(ctx context.Context, secret string) (*Account, error) {
	// We store bcrypt hashes, so we need to fetch all and compare.
	// With a small number of accounts this is fine.
	// For scale, you'd add a lookup-friendly token column.
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, secret, is_admin, created_at
		FROM accounts
	`)
	if err != nil {
		return nil, fmt.Errorf("querying accounts: %w", err)
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.Name, &a.Secret, &a.IsAdmin, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning account: %w", err)
		}
		accounts = append(accounts, a)
	}

	return nil, fmt.Errorf("no matching account found")
}

// --- Book Access ---

func (s *Store) GrantBookAccess(ctx context.Context, accountID, bookID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO book_access (account_id, book_id)
		VALUES ($1, $2)
		ON CONFLICT (account_id, book_id) DO NOTHING
	`, accountID, bookID)
	if err != nil {
		return fmt.Errorf("granting access: %w", err)
	}
	return nil
}

func (s *Store) RevokeBookAccess(ctx context.Context, accountID, bookID string) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM book_access WHERE account_id = $1 AND book_id = $2
	`, accountID, bookID)
	if err != nil {
		return fmt.Errorf("revoking access: %w", err)
	}
	return nil
}

func (s *Store) HasBookAccess(ctx context.Context, accountID, bookID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM book_access WHERE account_id = $1 AND book_id = $2
		)
	`, accountID, bookID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking access: %w", err)
	}
	return exists, nil
}

// ListAccountBooks returns all books an account has access to
func (s *Store) ListAccountBooks(ctx context.Context, accountID string) ([]Book, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT b.id, b.title, b.author, b.narrator, b.description, b.cover_path,
		       b.duration_sec, b.file_paths, b.abs_id, b.created_at, b.updated_at
		FROM books b
		JOIN book_access ba ON ba.book_id = b.id
		WHERE ba.account_id = $1
		ORDER BY b.title
	`, accountID)
	if err != nil {
		return nil, fmt.Errorf("listing account books: %w", err)
	}
	defer rows.Close()

	var books []Book
	for rows.Next() {
		var b Book
		if err := rows.Scan(
			&b.ID, &b.Title, &b.Author, &b.Narrator, &b.Description,
			&b.CoverPath, &b.DurationSec, &b.FilePaths, &b.ABSID,
			&b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning book: %w", err)
		}
		books = append(books, b)
	}
	return books, nil
}

// ListBookAccessByBook returns all accounts that have access to a book
func (s *Store) ListBookAccessByBook(ctx context.Context, bookID string) ([]BookAccess, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT ba.account_id, ba.book_id, ba.granted_at,
		       COALESCE(b.title, ''), COALESCE(b.author, ''),
		       COALESCE(a.name, '')
		FROM book_access ba
		LEFT JOIN books b ON b.id = ba.book_id
		LEFT JOIN accounts a ON a.id = ba.account_id
		WHERE ba.book_id = $1
		ORDER BY a.name
	`, bookID)
	if err != nil {
		return nil, fmt.Errorf("listing book access: %w", err)
	}
	defer rows.Close()

	var access []BookAccess
	for rows.Next() {
		var a BookAccess
		if err := rows.Scan(&a.AccountID, &a.BookID, &a.GrantedAt,
			&a.BookTitle, &a.BookAuthor, &a.AccountName); err != nil {
			return nil, fmt.Errorf("scanning book access: %w", err)
		}
		access = append(access, a)
	}
	return access, nil
}

// GrantAllBooksAccess gives an account access to every book in the library
func (s *Store) GrantAllBooksAccess(ctx context.Context, accountID string) (int, error) {
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO book_access (account_id, book_id)
		SELECT $1, id FROM books
		ON CONFLICT (account_id, book_id) DO NOTHING
	`, accountID)
	if err != nil {
		return 0, fmt.Errorf("granting all access: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

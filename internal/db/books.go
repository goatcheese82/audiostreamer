package db

import (
	"context"
	"fmt"
)

func (s *Store) ListBooks(ctx context.Context) ([]Book, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, title, author, narrator, description, cover_path, 
		       duration_sec, file_paths, abs_id, created_at, updated_at
		FROM books ORDER BY title
	`)
	if err != nil {
		return nil, fmt.Errorf("listing books: %w", err)
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

func (s *Store) GetBook(ctx context.Context, id string) (*Book, error) {
	var b Book
	err := s.pool.QueryRow(ctx, `
		SELECT id, title, author, narrator, description, cover_path,
		       duration_sec, file_paths, abs_id, created_at, updated_at
		FROM books WHERE id = $1
	`, id).Scan(
		&b.ID, &b.Title, &b.Author, &b.Narrator, &b.Description,
		&b.CoverPath, &b.DurationSec, &b.FilePaths, &b.ABSID,
		&b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting book %s: %w", id, err)
	}
	return &b, nil
}

func (s *Store) GetBookByTagUID(ctx context.Context, tagUID string) (*Book, error) {
	var b Book
	err := s.pool.QueryRow(ctx, `
		SELECT b.id, b.title, b.author, b.narrator, b.description, b.cover_path,
		       b.duration_sec, b.file_paths, b.abs_id, b.created_at, b.updated_at
		FROM books b
		JOIN nfc_tags t ON t.book_id = b.id
		WHERE t.tag_uid = $1
	`, tagUID).Scan(
		&b.ID, &b.Title, &b.Author, &b.Narrator, &b.Description,
		&b.CoverPath, &b.DurationSec, &b.FilePaths, &b.ABSID,
		&b.CreatedAt, &b.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting book by tag %s: %w", tagUID, err)
	}
	return &b, nil
}

func (s *Store) CreateBook(ctx context.Context, b *Book) error {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO books (title, author, narrator, description, cover_path, duration_sec, file_paths, abs_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at
	`, b.Title, b.Author, b.Narrator, b.Description, b.CoverPath,
		b.DurationSec, b.FilePaths, b.ABSID,
	).Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return fmt.Errorf("creating book: %w", err)
	}
	return nil
}

func (s *Store) UpdateBook(ctx context.Context, b *Book) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE books SET title=$2, author=$3, narrator=$4, description=$5,
		       cover_path=$6, duration_sec=$7, file_paths=$8, abs_id=$9, updated_at=now()
		WHERE id = $1
	`, b.ID, b.Title, b.Author, b.Narrator, b.Description,
		b.CoverPath, b.DurationSec, b.FilePaths, b.ABSID)
	if err != nil {
		return fmt.Errorf("updating book %s: %w", b.ID, err)
	}
	return nil
}

func (s *Store) DeleteBook(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM books WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting book %s: %w", id, err)
	}
	return nil
}

func (s *Store) UpsertBookByABSID(ctx context.Context, b *Book) error {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO books (title, author, narrator, description, cover_path, duration_sec, file_paths, abs_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (abs_id) WHERE abs_id != ''
		DO UPDATE SET title=$1, author=$2, narrator=$3, description=$4,
		              cover_path=$5, duration_sec=$6, file_paths=$7, updated_at=now()
		RETURNING id, created_at, updated_at
	`, b.Title, b.Author, b.Narrator, b.Description, b.CoverPath,
		b.DurationSec, b.FilePaths, b.ABSID,
	).Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upserting book by abs_id %s: %w", b.ABSID, err)
	}
	return nil
}

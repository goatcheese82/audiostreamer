package db

import (
	"context"
	"fmt"
)

func (s *Store) ListTags(ctx context.Context) ([]NFCTag, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.tag_uid, t.book_id, t.account_id, t.label, t.created_at,
		       COALESCE(b.title, ''), COALESCE(b.author, '')
		FROM nfc_tags t
		LEFT JOIN books b ON b.id = t.book_id
		ORDER BY t.created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}
	defer rows.Close()

	var tags []NFCTag
	for rows.Next() {
		var t NFCTag
		if err := rows.Scan(&t.TagUID, &t.BookID, &t.AccountID, &t.Label, &t.CreatedAt, &t.BookTitle, &t.BookAuthor); err != nil {
			return nil, fmt.Errorf("scanning tag: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, nil
}

func (s *Store) CreateTag(ctx context.Context, t *NFCTag) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO nfc_tags (tag_uid, book_id, account_id, label)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tag_uid) DO UPDATE SET book_id=$2, account_id=$3, label=$4
	`, t.TagUID, t.BookID, t.AccountID, t.Label)
	if err != nil {
		return fmt.Errorf("creating tag: %w", err)
	}
	return nil
}

func (s *Store) DeleteTag(ctx context.Context, tagUID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM nfc_tags WHERE tag_uid = $1`, tagUID)
	if err != nil {
		return fmt.Errorf("deleting tag %s: %w", tagUID, err)
	}
	return nil
}

func (s *Store) GetTag(ctx context.Context, tagUID string) (*NFCTag, error) {
	var t NFCTag
	err := s.pool.QueryRow(ctx, `
		SELECT t.tag_uid, t.book_id, t.account_id, t.label, t.created_at,
		       COALESCE(b.title, ''), COALESCE(b.author, '')
		FROM nfc_tags t
		LEFT JOIN books b ON b.id = t.book_id
		WHERE t.tag_uid = $1
	`, tagUID).Scan(&t.TagUID, &t.BookID, &t.AccountID, &t.Label, &t.CreatedAt, &t.BookTitle, &t.BookAuthor)
	if err != nil {
		return nil, fmt.Errorf("getting tag %s: %w", tagUID, err)
	}
	return &t, nil
}

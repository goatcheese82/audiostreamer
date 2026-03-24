package db

import (
	"context"
	"fmt"
)

func (s *Store) GetProgress(ctx context.Context, bookID, deviceID string) (*PlaybackProgress, error) {
	var p PlaybackProgress
	err := s.pool.QueryRow(ctx, `
		SELECT id, book_id, account_id, device_id, position_sec, is_finished, updated_at
		FROM playback_progress
		WHERE book_id = $1 AND device_id = $2
		ORDER BY updated_at DESC
		LIMIT 1
	`, bookID, deviceID).Scan(
		&p.ID, &p.BookID, &p.AccountID, &p.DeviceID, &p.PositionSec, &p.IsFinished, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting progress for book %s device %s: %w", bookID, deviceID, err)
	}
	return &p, nil
}

// GetProgressAnyDevice returns the most recently updated progress for a book
// within an account. Used when the ESP32 doesn't specify a device ID.
func (s *Store) GetProgressAnyDevice(ctx context.Context, bookID string) (*PlaybackProgress, error) {
	var p PlaybackProgress
	err := s.pool.QueryRow(ctx, `
		SELECT id, book_id, account_id, device_id, position_sec, is_finished, updated_at
		FROM playback_progress
		WHERE book_id = $1
		ORDER BY updated_at DESC
		LIMIT 1
	`, bookID).Scan(
		&p.ID, &p.BookID, &p.AccountID, &p.DeviceID, &p.PositionSec, &p.IsFinished, &p.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("getting progress for book %s: %w", bookID, err)
	}
	return &p, nil
}

func (s *Store) UpsertProgress(ctx context.Context, p *PlaybackProgress) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO playback_progress (book_id, account_id, device_id, position_sec, is_finished)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (book_id, account_id, device_id)
		DO UPDATE SET position_sec=$4, is_finished=$5, updated_at=now()
	`, p.BookID, p.AccountID, p.DeviceID, p.PositionSec, p.IsFinished)
	if err != nil {
		return fmt.Errorf("upserting progress: %w", err)
	}
	return nil
}

func (s *Store) UpsertDevice(ctx context.Context, d *Device) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO devices (device_id, account_id, name, last_seen, firmware_ver)
		VALUES ($1, $2, $3, now(), $4)
		ON CONFLICT (device_id)
		DO UPDATE SET account_id=$2,
		              name=COALESCE(NULLIF($3, ''), devices.name),
		              last_seen=now(),
		              firmware_ver=COALESCE(NULLIF($4, ''), devices.firmware_ver)
	`, d.DeviceID, d.AccountID, d.Name, d.FirmwareVer)
	if err != nil {
		return fmt.Errorf("upserting device: %w", err)
	}
	return nil
}

func (s *Store) ListDevices(ctx context.Context) ([]Device, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT device_id, account_id, name, last_seen, firmware_ver, created_at
		FROM devices ORDER BY last_seen DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("listing devices: %w", err)
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.DeviceID, &d.AccountID, &d.Name, &d.LastSeen, &d.FirmwareVer, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning device: %w", err)
		}
		devices = append(devices, d)
	}
	return devices, nil
}

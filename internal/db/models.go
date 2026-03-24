package db

import "time"

type Account struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Secret    string    `json:"secret,omitempty"` // bcrypt hash, omitted from most responses
	IsAdmin   bool      `json:"is_admin"`
	CreatedAt time.Time `json:"created_at"`
}

type Book struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Author      string    `json:"author"`
	Narrator    string    `json:"narrator"`
	Description string    `json:"description"`
	CoverPath   string    `json:"cover_path"`
	DurationSec int       `json:"duration_sec"`
	FilePaths   []string  `json:"file_paths"`
	ABSID       string    `json:"abs_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type NFCTag struct {
	TagUID    string    `json:"tag_uid"`
	BookID    string    `json:"book_id"`
	AccountID string    `json:"account_id"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"created_at"`
	// Joined fields
	BookTitle  string `json:"book_title,omitempty"`
	BookAuthor string `json:"book_author,omitempty"`
}

type PlaybackProgress struct {
	ID          string    `json:"id"`
	BookID      string    `json:"book_id"`
	AccountID   string    `json:"account_id"`
	DeviceID    string    `json:"device_id"`
	PositionSec float64   `json:"position_sec"`
	IsFinished  bool      `json:"is_finished"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Device struct {
	DeviceID    string    `json:"device_id"`
	AccountID   string    `json:"account_id"`
	Name        string    `json:"name"`
	LastSeen    time.Time `json:"last_seen"`
	FirmwareVer string    `json:"firmware_ver"`
	CreatedAt   time.Time `json:"created_at"`
}

type BookAccess struct {
	AccountID   string    `json:"account_id"`
	BookID      string    `json:"book_id"`
	GrantedAt   time.Time `json:"granted_at"`
	// Joined fields
	BookTitle   string    `json:"book_title,omitempty"`
	BookAuthor  string    `json:"book_author,omitempty"`
	AccountName string    `json:"account_name,omitempty"`
}

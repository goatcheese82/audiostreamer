# Audiostreamer

NFC-triggered audiobook streaming server. Tap an NFC tag on an ESP32 device to stream audiobooks from your library.

## Quick Start

### 1. Configure

```bash
cp .env.example .env
# Edit .env with your settings:
#   - AUDIOBOOK_BASE_PATH: path to your audiobooks on the Unraid mount
#   - ABS_URL / ABS_TOKEN: optional, for importing from Audiobookshelf
#   - DB_PASSWORD: pick a password for PostgreSQL
```

### 2. Run with Docker Compose

```bash
docker compose up -d
```

This starts:
- **audiostreamer** on port 8080 (Go API server)
- **db** PostgreSQL 16
- **admin** on port 3000 (SvelteKit admin UI)

### 3. Scan your library

```bash
# Scan the audiobook directory for books
curl -X POST http://10.0.2.166:8080/api/books/scan

# Or import from Audiobookshelf (requires ABS_TOKEN in .env)
curl -X POST http://10.0.2.166:8080/api/books/import
```

### 4. Assign NFC tags

Open the admin UI at `http://10.0.2.166:3000` and map NFC tags to books.

Or via API:
```bash
# List books to find the book ID
curl http://10.0.2.166:8080/api/books | jq

# Assign a tag
curl -X POST http://10.0.2.166:8080/api/tags \
  -H "Content-Type: application/json" \
  -d '{"tag_uid": "04A32B1C5E8000", "book_id": "YOUR-BOOK-UUID", "label": "Blue tag"}'
```

### 5. Test streaming

```bash
# Stream with mpv (or vlc, ffplay, etc.)
mpv http://10.0.2.166:8080/api/play/04A32B1C5E8000

# Or save a clip to verify transcoding
curl -s http://10.0.2.166:8080/api/play/04A32B1C5E8000 | head -c 100000 > test.ogg
ffprobe test.ogg
```

## Development (without Docker)

### Prerequisites

- Go 1.22+
- PostgreSQL 16
- ffmpeg with libopus

### Run locally

```bash
# Start PostgreSQL (or use existing instance)
createdb audiostreamer

# Set env vars
export DATABASE_URL="postgresql://localhost:5432/audiostreamer?sslmode=disable"
export AUDIOBOOK_BASE_PATH="/mnt/tower/audiobooks"

# Build and run
go mod tidy
go run ./cmd/server
```

## API Reference

### ESP32 Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/play/:nfc_id` | Stream audio (OGG/Opus), resumes from last position |
| `GET` | `/api/book/:nfc_id` | Get book metadata + progress |
| `POST` | `/api/progress/:nfc_id` | Save playback position |
| `POST` | `/api/stop/:nfc_id` | Mark playback stopped |
| `POST` | `/api/tags/register` | Register an unknown tag |

### Admin Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/books` | List all books |
| `POST` | `/api/books` | Create a book |
| `GET` | `/api/books/:id` | Get a book |
| `PUT` | `/api/books/:id` | Update a book |
| `DELETE` | `/api/books/:id` | Delete a book |
| `POST` | `/api/books/scan` | Scan directory for books |
| `POST` | `/api/books/import` | Import from Audiobookshelf |
| `GET` | `/api/tags` | List all tag mappings |
| `POST` | `/api/tags` | Create tag → book mapping |
| `DELETE` | `/api/tags/:tag_uid` | Remove tag mapping |
| `GET` | `/api/devices` | List ESP32 devices |

### Query Parameters

- `device` — ESP32 MAC address, used for per-device progress tracking
- `pos` — Override start position in seconds (skip resume logic)

## Architecture

See `nfc-audiobook-streamer-architecture.md` for the full system design.

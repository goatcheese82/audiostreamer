package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port             int
	DatabaseURL      string
	AudiobookBasePath string
	ABSUrl           string
	ABSToken         string
	AdminToken       string
	FFmpegPath       string
	OpusBitrate      string
	SampleRate       int
}

func Load() (*Config, error) {
	port, _ := strconv.Atoi(getEnv("PORT", "8080"))
	sampleRate, _ := strconv.Atoi(getEnv("SAMPLE_RATE", "48000"))

	cfg := &Config{
		Port:             port,
		DatabaseURL:      getEnv("DATABASE_URL", "postgresql://audiostreamer:audiostreamer@localhost:5432/audiostreamer?sslmode=disable"),
		AudiobookBasePath: getEnv("AUDIOBOOK_BASE_PATH", "/mnt/tower/audiobooks"),
		ABSUrl:           getEnv("ABS_URL", "http://10.0.2.8:13378"),
		ABSToken:         getEnv("ABS_TOKEN", ""),
		AdminToken:       getEnv("ADMIN_TOKEN", ""),
		FFmpegPath:       getEnv("FFMPEG_PATH", "ffmpeg"),
		OpusBitrate:      getEnv("OPUS_BITRATE", "48k"),
		SampleRate:       sampleRate,
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

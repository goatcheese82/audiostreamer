package stream

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Transcoder struct {
	FFmpegPath string
	Bitrate    string
	SampleRate int
}

func NewTranscoder(ffmpegPath, bitrate string, sampleRate int) *Transcoder {
	return &Transcoder{
		FFmpegPath: ffmpegPath,
		Bitrate:    bitrate,
		SampleRate: sampleRate,
	}
}

// StreamResult holds the ffmpeg process and its stdout for streaming
type StreamResult struct {
	cmd    *exec.Cmd
	Reader io.ReadCloser
}

func (sr *StreamResult) Close() error {
	if sr.Reader != nil {
		sr.Reader.Close()
	}
	if sr.cmd != nil && sr.cmd.Process != nil {
		sr.cmd.Process.Kill()
		sr.cmd.Wait()
	}
	return nil
}

// StreamSingle transcodes a single audio file starting at the given position
func (t *Transcoder) StreamSingle(ctx context.Context, filePath string, seekSec float64) (*StreamResult, error) {
	args := []string{
		"-hide_banner",
		"-loglevel", "warning",
	}

	// Input seeking (fast, placed before -i)
	if seekSec > 0 {
		args = append(args, "-ss", formatSeconds(seekSec))
	}

	args = append(args,
		"-i", filePath,
		"-c:a", "libopus",
		"-b:a", t.Bitrate,
		"-ar", strconv.Itoa(t.SampleRate),
		"-ac", "1",
		"-vn",        // strip cover art / video
		"-f", "ogg",
		"pipe:1",
	)

	cmd := exec.CommandContext(ctx, t.FFmpegPath, args...)
	cmd.Stderr = os.Stderr // log ffmpeg errors

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting ffmpeg: %w", err)
	}

	log.Printf("[transcode] streaming %s from %.1fs", filepath.Base(filePath), seekSec)

	return &StreamResult{cmd: cmd, Reader: stdout}, nil
}

// StreamConcat transcodes multiple audio files concatenated, starting at the given
// global position (seconds from the start of the first file).
func (t *Transcoder) StreamConcat(ctx context.Context, filePaths []string, seekSec float64) (*StreamResult, error) {
	if len(filePaths) == 0 {
		return nil, fmt.Errorf("no files to stream")
	}

	if len(filePaths) == 1 {
		return t.StreamSingle(ctx, filePaths[0], seekSec)
	}

	// Build a concat demuxer file
	concatFile, err := os.CreateTemp("", "audiostreamer-concat-*.txt")
	if err != nil {
		return nil, fmt.Errorf("creating concat file: %w", err)
	}

	for _, fp := range filePaths {
		// ffmpeg concat demuxer requires escaped single quotes in paths
		escaped := strings.ReplaceAll(fp, "'", "'\\''")
		fmt.Fprintf(concatFile, "file '%s'\n", escaped)
	}
	concatFile.Close()

	args := []string{
		"-hide_banner",
		"-loglevel", "warning",
		"-f", "concat",
		"-safe", "0",
	}

	if seekSec > 0 {
		args = append(args, "-ss", formatSeconds(seekSec))
	}

	args = append(args,
		"-i", concatFile.Name(),
		"-c:a", "libopus",
		"-b:a", t.Bitrate,
		"-ar", strconv.Itoa(t.SampleRate),
		"-ac", "1",
		"-vn",
		"-f", "ogg",
		"pipe:1",
	)

	cmd := exec.CommandContext(ctx, t.FFmpegPath, args...)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		os.Remove(concatFile.Name())
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		os.Remove(concatFile.Name())
		return nil, fmt.Errorf("starting ffmpeg: %w", err)
	}

	log.Printf("[transcode] streaming %d files from %.1fs", len(filePaths), seekSec)

	// Clean up concat file when process exits
	go func() {
		cmd.Wait()
		os.Remove(concatFile.Name())
	}()

	return &StreamResult{cmd: cmd, Reader: stdout}, nil
}

func formatSeconds(sec float64) string {
	h := int(sec) / 3600
	m := (int(sec) % 3600) / 60
	s := sec - float64(h*3600+m*60)
	return fmt.Sprintf("%d:%02d:%06.3f", h, m, s)
}

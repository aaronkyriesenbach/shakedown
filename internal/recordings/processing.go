package recordings

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ffprobeResult holds output parsed from ffprobe JSON.
type ffprobeResult struct {
	Format struct {
		Duration string            `json:"duration"`
		BitRate  string            `json:"bit_rate"`
		Tags     map[string]string `json:"tags"`
	} `json:"format"`
	Streams []struct {
		CodecType  string `json:"codec_type"`
		SampleRate string `json:"sample_rate"`
		Channels   int    `json:"channels"`
	} `json:"streams"`
}

// runFFprobe runs ffprobe on the given file and returns parsed metadata.
func runFFprobe(ctx context.Context, filePath string) (*ffprobeResult, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe: failed to run: %w", err)
	}

	var result ffprobeResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("ffprobe: failed to parse output: %w", err)
	}
	return &result, nil
}

// runFFmpeg transcodes input to AAC 192kbps M4A with faststart.
func runFFmpeg(ctx context.Context, inputPath, outputPath string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", inputPath,
		"-c:a", "aac",
		"-b:a", "192k",
		"-movflags", "+faststart",
		"-y", // overwrite
		outputPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg: failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// runAudiowaveform generates waveform peaks JSON using BBC audiowaveform.
// It first converts the input to WAV via ffmpeg since audiowaveform only
// supports WAV, MP3, FLAC, and Ogg natively.
func runAudiowaveform(ctx context.Context, inputPath, outputPath string) error {
	wavPath := outputPath + ".tmp.wav"
	defer os.Remove(wavPath)

	ffCmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", inputPath,
		"-ac", "1",
		"-ar", "16000",
		"-y",
		wavPath,
	)
	if out, err := ffCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("audiowaveform: wav conversion failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	cmd := exec.CommandContext(ctx, "audiowaveform",
		"--input-filename", wavPath,
		"--output-filename", outputPath,
		"--output-format", "json",
		"--pixels-per-second", "10",
		"--bits", "8",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("audiowaveform: failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ProcessingJob holds the data needed to process a recording.
type ProcessingJob struct {
	RecordingID string
	StorageRoot string
	FileExt     string
}

// processRecording runs the full pipeline: ffprobe -> ffmpeg -> audiowaveform.
// It updates the DB record with results at each stage.
// Errors are logged but do not crash the server.
func (svc *Service) processRecording(ctx context.Context, job ProcessingJob) {
	recordingDir := filepath.Join(job.StorageRoot, job.RecordingID)
	originalPath := filepath.Join(recordingDir, "original"+job.FileExt)
	playbackPath := filepath.Join(recordingDir, "playback.m4a")
	waveformPath := filepath.Join(recordingDir, "waveform.json")

	var (
		durationSeconds float64
		bitrate         int
		sampleRate      int
		channels        int
		playbackReady   bool
		waveformReady   bool
		procErr         *string
	)

	_ = svc.repo.UpdateProcessingStep(ctx, job.RecordingID, "analyzing")

	probeResult, err := runFFprobe(ctx, originalPath)
	if err != nil {
		errStr := err.Error()
		procErr = &errStr
		_ = svc.repo.UpdateProcessingResult(ctx, job.RecordingID,
			0, 0, 0, 0, false, false, procErr)
		return
	}

	if d, err := strconv.ParseFloat(probeResult.Format.Duration, 64); err == nil {
		durationSeconds = d
	}
	if b, err := strconv.Atoi(probeResult.Format.BitRate); err == nil {
		bitrate = b
	}
	for _, stream := range probeResult.Streams {
		if stream.CodecType == "audio" {
			if sr, err := strconv.Atoi(stream.SampleRate); err == nil {
				sampleRate = sr
			}
			channels = stream.Channels
			break
		}
	}

	_ = svc.repo.UpdateProcessingStep(ctx, job.RecordingID, "transcoding")

	if err := runFFmpeg(ctx, originalPath, playbackPath); err != nil {
		errStr := err.Error()
		procErr = &errStr
		_ = svc.repo.UpdateProcessingResult(ctx, job.RecordingID,
			durationSeconds, bitrate, sampleRate, channels, false, false, procErr)
		return
	}
	playbackReady = true

	_ = svc.repo.UpdateProcessingStep(ctx, job.RecordingID, "generating_waveform")

	if err := runAudiowaveform(ctx, originalPath, waveformPath); err != nil {
		errStr := fmt.Sprintf("waveform generation failed: %v", err)
		procErr = &errStr
	} else {
		waveformReady = true
		procErr = nil
	}

	_ = svc.repo.UpdateProcessingResult(ctx, job.RecordingID,
		durationSeconds, bitrate, sampleRate, channels, playbackReady, waveformReady, procErr)
}

// parseDateFromTags tries to extract recorded_at from ffprobe tags.
// Returns zero time if no date tags found.
func parseDateFromTags(tags map[string]string) time.Time {
	for _, key := range []string{"date", "creation_time", "com.apple.quicktime.creationdate"} {
		if val, ok := tags[key]; ok {
			for _, layout := range []string{
				time.RFC3339,
				"2006-01-02T15:04:05",
				"2006-01-02",
			} {
				if t, err := time.Parse(layout, val); err == nil {
					return t.UTC()
				}
			}
		}
	}
	return time.Time{}
}

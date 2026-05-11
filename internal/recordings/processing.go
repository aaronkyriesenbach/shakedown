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
		Width      int    `json:"width"`
		Height     int    `json:"height"`
		CodecName  string `json:"codec_name"`
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

// runFFmpegVideo transcodes input to H.264/AAC MP4 with a 1080p cap.
func runFFmpegVideo(ctx context.Context, inputPath, outputPath string, sourceHeight int) error {
	args := []string{"-i", inputPath}

	if sourceHeight > 1080 {
		args = append(args, "-vf", "scale=-2:1080")
		args = append(args, "-c:v", "libx264", "-preset", "medium", "-crf", "23")
	} else {
		args = append(args, "-c:v", "libx264", "-preset", "medium", "-crf", "23")
	}

	args = append(args,
		"-c:a", "aac",
		"-b:a", "192k",
		"-movflags", "+faststart",
		"-y",
		outputPath,
	)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg video: failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func extractAudioFromVideo(ctx context.Context, inputPath, outputPath string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", inputPath,
		"-vn",
		"-c:a", "aac",
		"-b:a", "192k",
		"-movflags", "+faststart",
		"-y",
		outputPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg audio extract: failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// extractThumbnail extracts a representative JPEG frame from a video.
func extractThumbnail(ctx context.Context, inputPath, outputPath string, durationSeconds float64) error {
	seek := durationSeconds * 0.1
	if seek < 0.1 {
		seek = 0.1
	}

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-ss", fmt.Sprintf("%.3f", seek),
		"-i", inputPath,
		"-vframes", "1",
		"-vf", "scale=640:-2",
		"-q:v", "2",
		"-y",
		outputPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg thumbnail: failed: %w (output: %s)", err, strings.TrimSpace(string(out)))
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
	MediaType   string
}

// PlaybackFilename returns the playback file name for the given media type.
func PlaybackFilename(mediaType string) string {
	if mediaType == "video" {
		return "playback.mp4"
	}
	return "playback.m4a"
}

// AudioExtractFilename returns the audio-only extract filename for video recordings.
func AudioExtractFilename() string {
	return "audio.m4a"
}

// SnippetFilename returns the snippet file name for the given media type.
func SnippetFilename(mediaType string) string {
	if mediaType == "video" {
		return "snippet.mp4"
	}
	return "snippet.m4a"
}

// processRecording runs the appropriate processing pipeline for the media type.
func (svc *Service) processRecording(ctx context.Context, job ProcessingJob) {
	recordingDir := filepath.Join(job.StorageRoot, job.RecordingID)
	originalPath := filepath.Join(recordingDir, "original"+job.FileExt)

	if job.MediaType == "video" {
		svc.processVideoRecording(ctx, job, recordingDir, originalPath)
		return
	}

	svc.processAudioRecording(ctx, job, recordingDir, originalPath)
}

// processAudioRecording runs the full pipeline: ffprobe -> ffmpeg -> audiowaveform.
// It updates the DB record with results at each stage.
// Errors are logged but do not crash the server.
func (svc *Service) processAudioRecording(ctx context.Context, job ProcessingJob, recordingDir, originalPath string) {
	playbackPath := filepath.Join(recordingDir, PlaybackFilename("audio"))
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
			0, 0, 0, 0, false, false, procErr, false, false, nil, nil)
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
			durationSeconds, bitrate, sampleRate, channels, false, false, procErr, false, false, nil, nil)
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
		durationSeconds, bitrate, sampleRate, channels, playbackReady, waveformReady, procErr, false, false, nil, nil)
}

func (svc *Service) processVideoRecording(ctx context.Context, job ProcessingJob, recordingDir, originalPath string) {
	playbackPath := filepath.Join(recordingDir, PlaybackFilename("video"))
	thumbnailPath := filepath.Join(recordingDir, "thumbnail.jpg")
	audioExtractPath := filepath.Join(recordingDir, AudioExtractFilename())
	waveformPath := filepath.Join(recordingDir, "waveform.json")

	var (
		durationSeconds   float64
		bitrate           int
		sampleRate        int
		channels          int
		videoWidth        int
		videoHeight       int
		playbackReady     bool
		thumbnailReady    bool
		audioExtractReady bool
		waveformReady     bool
		procErr           *string
	)

	_ = svc.repo.UpdateProcessingStep(ctx, job.RecordingID, "analyzing")

	probeResult, err := runFFprobe(ctx, originalPath)
	if err != nil {
		errStr := err.Error()
		procErr = &errStr
		_ = svc.repo.UpdateProcessingResult(ctx, job.RecordingID,
			0, 0, 0, 0, false, false, procErr, false, false, nil, nil)
		return
	}

	if d, err := strconv.ParseFloat(probeResult.Format.Duration, 64); err == nil {
		durationSeconds = d
	}
	if b, err := strconv.Atoi(probeResult.Format.BitRate); err == nil {
		bitrate = b
	}
	for _, stream := range probeResult.Streams {
		switch stream.CodecType {
		case "audio":
			if sr, err := strconv.Atoi(stream.SampleRate); err == nil {
				sampleRate = sr
			}
			channels = stream.Channels
		case "video":
			videoWidth = stream.Width
			videoHeight = stream.Height
		}
	}

	_ = svc.repo.UpdateProcessingStep(ctx, job.RecordingID, "transcoding")

	if err := runFFmpegVideo(ctx, originalPath, playbackPath, videoHeight); err != nil {
		errStr := err.Error()
		procErr = &errStr
		_ = svc.repo.UpdateProcessingResult(ctx, job.RecordingID,
			durationSeconds, bitrate, sampleRate, channels, false, false, procErr, false, false, nil, nil)
		return
	}
	playbackReady = true

	_ = svc.repo.UpdateProcessingStep(ctx, job.RecordingID, "extracting_thumbnail")

	if err := extractThumbnail(ctx, originalPath, thumbnailPath, durationSeconds); err != nil {
		errStr := fmt.Sprintf("thumbnail extraction failed: %v", err)
		procErr = &errStr
	} else {
		thumbnailReady = true
		procErr = nil
	}

	_ = svc.repo.UpdateProcessingStep(ctx, job.RecordingID, "extracting_audio")

	if err := extractAudioFromVideo(ctx, playbackPath, audioExtractPath); err != nil {
		errStr := fmt.Sprintf("audio extraction failed: %v", err)
		procErr = &errStr
	} else {
		audioExtractReady = true
		procErr = nil
	}

	_ = svc.repo.UpdateProcessingStep(ctx, job.RecordingID, "generating_waveform")

	if err := runAudiowaveform(ctx, audioExtractPath, waveformPath); err != nil {
		errStr := fmt.Sprintf("waveform generation failed: %v", err)
		procErr = &errStr
	} else {
		waveformReady = true
		procErr = nil
	}

	vw := videoWidth
	vh := videoHeight
	_ = svc.repo.UpdateProcessingResult(ctx, job.RecordingID,
		durationSeconds, bitrate, sampleRate, channels, playbackReady, waveformReady, procErr, thumbnailReady, audioExtractReady, &vw, &vh)
}

// parseDateFromTags tries to extract recorded_at from ffprobe tags.
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

// ExtractSegment extracts a section of a media file into a snippet.
// inputPath is the source playback file. outputDir will contain the snippet file and,
// for audio media types, waveform.json. mediaType should be "audio" or "video":
// audio uses AAC re-encode and generates a waveform; video uses stream copy and skips waveform.
func ExtractSegment(ctx context.Context, inputPath, outputDir string, startSec, endSec float64, mediaType string) error {
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return fmt.Errorf("extract segment: mkdir: %w", err)
	}

	snippetPath := filepath.Join(outputDir, SnippetFilename(mediaType))
	duration := endSec - startSec

	var cmd *exec.Cmd
	if mediaType == "video" {
		cmd = exec.CommandContext(ctx, "ffmpeg", "-y",
			"-ss", strconv.FormatFloat(startSec, 'f', 3, 64),
			"-i", inputPath,
			"-t", strconv.FormatFloat(duration, 'f', 3, 64),
			"-c:v", "copy", "-c:a", "copy",
			"-movflags", "+faststart",
			snippetPath,
		)
	} else {
		cmd = exec.CommandContext(ctx, "ffmpeg", "-y",
			"-ss", strconv.FormatFloat(startSec, 'f', 3, 64),
			"-i", inputPath,
			"-t", strconv.FormatFloat(duration, 'f', 3, 64),
			"-c:a", "aac", "-b:a", "192k",
			"-movflags", "+faststart",
			snippetPath,
		)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("extract segment: ffmpeg: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	if mediaType != "video" {
		waveformPath := filepath.Join(outputDir, "waveform.json")
		if err := runAudiowaveform(ctx, snippetPath, waveformPath); err != nil {
			return fmt.Errorf("extract segment: waveform: %w", err)
		}
	}

	return nil
}

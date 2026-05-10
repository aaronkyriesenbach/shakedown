package recordings

import (
	"bytes"
	"io"
	"testing"
)

func TestValidateMediaMagicBytes(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		wantMime string
		wantExt  string
		wantErr  bool
	}{
		{
			name:     "MP3 ID3v2",
			data:     []byte{0x49, 0x44, 0x33, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			wantMime: "audio/mpeg", wantExt: ".mp3",
		},
		{
			name:     "MP3 sync",
			data:     []byte{0xFF, 0xFB, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			wantMime: "audio/mpeg", wantExt: ".mp3",
		},
		{
			name:     "FLAC",
			data:     []byte{0x66, 0x4C, 0x61, 0x43, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			wantMime: "audio/flac", wantExt: ".flac",
		},
		{
			name:     "WAV",
			data:     append([]byte{0x52, 0x49, 0x46, 0x46}, append([]byte{0x00, 0x00, 0x00, 0x00}, []byte{0x57, 0x41, 0x56, 0x45}...)...),
			wantMime: "audio/wav", wantExt: ".wav",
		},
		{
			name:     "OGG",
			data:     []byte{0x4F, 0x67, 0x67, 0x53, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			wantMime: "audio/ogg", wantExt: ".ogg",
		},
		{
			name:     "M4A (ftyp M4A )",
			data:     buildFtypBox("M4A "),
			wantMime: "audio/mp4", wantExt: ".m4a",
		},
		{
			name:     "MP4 (ftyp isom)",
			data:     buildFtypBox("isom"),
			wantMime: "video/mp4", wantExt: ".mp4",
		},
		{
			name:     "MP4 (ftyp mp42)",
			data:     buildFtypBox("mp42"),
			wantMime: "video/mp4", wantExt: ".mp4",
		},
		{
			name:     "MP4 (ftyp avc1)",
			data:     buildFtypBox("avc1"),
			wantMime: "video/mp4", wantExt: ".mp4",
		},
		{
			name:     "MOV (ftyp qt  )",
			data:     buildFtypBox("qt  "),
			wantMime: "video/quicktime", wantExt: ".mov",
		},
		{
			name:    "unknown format",
			data:    []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := tt.data
			for len(data) < 12 {
				data = append(data, 0x00)
			}
			mime, ext, r, err := ValidateMediaMagicBytes(bytes.NewReader(data))
			if tt.wantErr {
				if err == nil {
					t.Error("expected error for unknown format, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mime != tt.wantMime {
				t.Errorf("mime: got %q, want %q", mime, tt.wantMime)
			}
			if ext != tt.wantExt {
				t.Errorf("ext: got %q, want %q", ext, tt.wantExt)
			}
			got, err := io.ReadAll(r)
			if err != nil {
				t.Fatalf("failed to read returned reader: %v", err)
			}
			if len(got) == 0 {
				t.Error("returned reader was empty")
			}
		})
	}
}

// buildFtypBox builds a minimal valid ftyp box with the given 4-byte brand.
// Structure: size(4) "ftyp"(4) brand(4) version(4) — total 16 bytes
func buildFtypBox(brand string) []byte {
	b := make([]byte, 16)
	b[0], b[1], b[2], b[3] = 0x00, 0x00, 0x00, 0x10
	b[4], b[5], b[6], b[7] = 'f', 't', 'y', 'p'
	copy(b[8:12], []byte(brand))
	return b
}

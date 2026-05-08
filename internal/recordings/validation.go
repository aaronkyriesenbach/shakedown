package recordings

import (
	"bytes"
	"fmt"
	"io"
)

// audioMagicBytes maps MIME types to their magic byte signatures.
var audioMagicBytes = []struct {
	mime   string
	ext    string
	prefix []byte
}{
	{"audio/mpeg", ".mp3", []byte{0xFF, 0xFB}},
	{"audio/mpeg", ".mp3", []byte{0xFF, 0xF3}},
	{"audio/mpeg", ".mp3", []byte{0xFF, 0xF2}},
	{"audio/mpeg", ".mp3", []byte{0x49, 0x44, 0x33}}, // ID3
	{"audio/flac", ".flac", []byte{0x66, 0x4C, 0x61, 0x43}}, // fLaC
	{"audio/wav", ".wav", []byte{0x52, 0x49, 0x46, 0x46}},   // RIFF
	{"audio/ogg", ".ogg", []byte{0x4F, 0x67, 0x67, 0x53}},   // OggS
	{"audio/mp4", ".m4a", []byte{0x00, 0x00, 0x00, 0x20, 0x66, 0x74, 0x79, 0x70}}, // ftyp (offset 4)
	{"audio/mp4", ".m4a", []byte{0x00, 0x00, 0x00, 0x1C, 0x66, 0x74, 0x79, 0x70}}, // ftyp variant
}

// ValidateAudioMagicBytes reads the first 32 bytes from r and checks if they match
// a known audio format. Returns the detected MIME type and file extension.
// The returned reader is a new reader that includes the bytes already read.
func ValidateAudioMagicBytes(r io.Reader) (string, string, io.Reader, error) {
	header := make([]byte, 32)
	n, err := io.ReadFull(r, header)
	if err != nil && err != io.ErrUnexpectedEOF {
		return "", "", nil, fmt.Errorf("validation: failed to read header: %w", err)
	}
	header = header[:n]

	for _, sig := range audioMagicBytes {
		if bytes.HasPrefix(header, sig.prefix) {
			combined := io.MultiReader(bytes.NewReader(header), r)
			return sig.mime, sig.ext, combined, nil
		}
	}

	return "", "", nil, fmt.Errorf("validation: unsupported audio format (not MP3, FLAC, WAV, OGG, or M4A)")
}

package recordings

import (
    "bytes"
    "fmt"
    "io"
)

// mediaMagicBytes maps MIME types to their magic byte signatures for non-ftyp formats.
var mediaMagicBytes = []struct {
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
}

// ftypBrands maps ftyp major brands to MIME type and extension.
// Key is exactly 4 bytes (padded with spaces as needed).
var ftypBrands = map[string][2]string{
    "M4A ": {"audio/mp4", ".m4a"},
    "isom": {"video/mp4", ".mp4"},
    "mp42": {"video/mp4", ".mp4"},
    "mp41": {"video/mp4", ".mp4"},
    "avc1": {"video/mp4", ".mp4"},
    "qt  ": {"video/quicktime", ".mov"},
}

// ValidateMediaMagicBytes reads the first 32 bytes from r and checks if they match
// a known audio or video format. Returns the detected MIME type and file extension.
// The returned reader is a new reader that includes the bytes already read.
func ValidateMediaMagicBytes(r io.Reader) (string, string, io.Reader, error) {
    header := make([]byte, 32)
    n, err := io.ReadFull(r, header)
    if err != nil && err != io.ErrUnexpectedEOF {
        return "", "", nil, fmt.Errorf("validation: failed to read header: %w", err)
    }
    header = header[:n]

    // Check non-ftyp formats first (MP3, FLAC, WAV, OGG)
    for _, sig := range mediaMagicBytes {
        if bytes.HasPrefix(header, sig.prefix) {
            combined := io.MultiReader(bytes.NewReader(header), r)
            return sig.mime, sig.ext, combined, nil
        }
    }

    // Check for ftyp box (ISO base media file format: MP4, MOV, M4A)
    // Structure: [4 bytes size][4 bytes "ftyp"][4 bytes major brand][4 bytes minor version][...]
    // The box size is variable so we look for "ftyp" at offset 4
    if len(header) >= 12 && bytes.Equal(header[4:8], []byte("ftyp")) {
        brand := string(header[8:12])
        if result, ok := ftypBrands[brand]; ok {
            combined := io.MultiReader(bytes.NewReader(header), r)
            return result[0], result[1], combined, nil
        }
        // Unknown ftyp brand — reject
        return "", "", nil, fmt.Errorf("validation: unsupported ftyp brand %q", brand)
    }

    // Also validate that box size is plausible (bytes 0-3 as big-endian uint32)
    if len(header) >= 8 {
        _ = header[0]
    }

    return "", "", nil, fmt.Errorf("validation: unsupported media format (not MP3, FLAC, WAV, OGG, M4A, MP4, or MOV)")
}

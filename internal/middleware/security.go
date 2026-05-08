// Security audit findings (Wave 8.3):
// - Path traversal:  PASS — LocalStorage.SafeJoin in recordings/storage.go uses filepath.Clean("/"+subPath)
//                    and verifies the result still has the storage root as a prefix (+ separator).
//                    All Write/Read/Delete/Exists/MkdirAll calls go through SafeJoin.
// - SQL injection:   PASS — all queries across repository files use $1, $2, … parameterized placeholders
//                    enforced by pgx/v5; no string interpolation in SQL found.
// - CSRF:            PASS — shakedown_session cookie is set with SameSite=Strict (auth/handler.go).
//                    OIDC state cookie is compared before exchange (callback validates stateCookie.Value
//                    against the ?state= query parameter). oidc_state/oidc_nonce use SameSite=Lax,
//                    which is correct for the cross-site OIDC redirect flow.
// - Share tokens:    PASS — crypto/rand generates 32 random bytes, base64url-encoded (shares/repository.go).
// - Magic bytes:     PASS — ValidateAudioMagicBytes in recordings/validation.go inspects the first 32 bytes
//                    before any file is accepted; unsupported formats are rejected with 422.
// - Comment XSS:     PASS — comments are stored as plain TEXT in PostgreSQL; React's JSX rendering
//                    escapes all output on the frontend — no server-side HTML encoding required.
// - Upload size:     PASS — http.MaxBytesReader applied to r.Body before ParseMultipartForm
//                    in recordings/handler.go using cfg.UploadMaxSizeMB.

package middleware

import "net/http"

// SecurityHeaders adds security-related HTTP response headers to every request.
// Mount this early in the middleware stack (after RequestID/RealIP/Recoverer/Logger)
// so all routes — API and static file server — receive the headers.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// CSP: allow self + inline styles (Tailwind utility classes injected at runtime),
		// blob: and data: for waveform canvas/audio, frame-ancestors 'none' mirrors X-Frame-Options.
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; "+
				"img-src 'self' data: blob:; media-src 'self' blob:; connect-src 'self'; "+
				"font-src 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

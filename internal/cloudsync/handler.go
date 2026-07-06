package cloudsync

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"shakedown/internal/auth"
)

// maxRemoteConfigBytes bounds the size of a pasted rclone remote-config body.
const maxRemoteConfigBytes = 1 << 20 // 1MB

// StatusFunc reports the FULL readiness chain: flag on, remote name set,
// template valid, rclone binary present, AND remote reachable. Used to gate
// GET /status and POST /run, where "nothing to sync to yet" is a legitimate
// 409. It must NOT gate /test or /remote — see enabledFn below.
type StatusFunc func() (enabled bool, reason string)

// Handler exposes the cloud-sync admin API. Routes are NOT mounted by this
// package — Todo 11's main.go mounts Handler.Routes under /admin/cloud-sync
// (or similar) behind its own auth wiring.
type Handler struct {
	svc        *Service
	client     RemoteClient
	store      StateStore
	remoteName string
	statusFn   StatusFunc
	// enabledFn reports ONLY the CLOUD_SYNC_ENABLED feature flag (no remote
	// reachability check). /test and /remote gate on this instead of
	// statusFn: their entire purpose is to help an admin go from "remote not
	// configured yet" to "fully working", so they must stay usable even
	// while the full readiness probe (statusFn) still reports disabled.
	enabledFn func() bool
	logger    *zap.Logger

	mu              sync.Mutex
	lastReconcileAt *time.Time
}

func NewHandler(svc *Service, client RemoteClient, store StateStore, remoteName string, statusFn StatusFunc, enabledFn func() bool, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	if statusFn == nil {
		statusFn = func() (bool, string) { return false, "cloud sync not configured" }
	}
	if enabledFn == nil {
		enabledFn = func() bool { return false }
	}
	return &Handler{
		svc:        svc,
		client:     client,
		store:      store,
		remoteName: remoteName,
		statusFn:   statusFn,
		enabledFn:  enabledFn,
		logger:     logger,
	}
}

// Routes registers the cloud-sync admin endpoints on r. Each route
// self-applies auth.RequireAdmin, mirroring internal/admin/handler.go's
// defensive-layering convention (still expected to be mounted behind an
// outer requireAuth group by the caller).
func (h *Handler) Routes(r chi.Router) {
	r.With(auth.RequireAdmin).Get("/status", h.status)
	r.With(auth.RequireAdmin).Post("/run", h.run)
	r.With(auth.RequireAdmin).Post("/test", h.test)
	r.With(auth.RequireAdmin).Post("/remote", h.remote)
}

type statusCounts struct {
	Total   int `json:"total"`
	Synced  int `json:"synced"`
	Pending int `json:"pending"`
	Syncing int `json:"syncing"`
	Error   int `json:"error"`
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	enabled, reason := h.statusFn()
	if !enabled {
		writeJSON(w, http.StatusOK, map[string]any{
			"enabled": false,
			"reason":  reason,
		})
		return
	}

	counts, err := h.store.CountByStatus(r.Context())
	if err != nil {
		h.logger.Error("status: CountByStatus failed", zap.Error(err))
		counts = map[string]int{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":           true,
		"reason":            reason,
		"remote_configured": h.remoteName != "",
		"remote_name":       h.remoteName,
		"counts": statusCounts{
			Total:   counts["pending"] + counts["synced"] + counts["syncing"] + counts["error"],
			Synced:  counts["synced"],
			Pending: counts["pending"],
			Syncing: counts["syncing"],
			Error:   counts["error"],
		},
		"in_progress":       counts["syncing"] > 0,
		"last_reconcile_at": h.getLastReconcileAt(),
	})
}

func (h *Handler) run(w http.ResponseWriter, r *http.Request) {
	enabled, reason := h.statusFn()
	if !enabled {
		writeJSONError(w, http.StatusConflict, reason)
		return
	}

	go func() {
		_ = h.svc.Reconcile(context.Background())
		now := time.Now()
		h.mu.Lock()
		h.lastReconcileAt = &now
		h.mu.Unlock()
	}()

	w.WriteHeader(http.StatusAccepted)
}

func (h *Handler) test(w http.ResponseWriter, r *http.Request) {
	if !h.enabledFn() {
		writeJSONError(w, http.StatusConflict, "cloud sync is not enabled")
		return
	}

	ok, err := h.client.RemoteExists(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "reason": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "reason": "remote not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// remote accepts a pasted rclone remote-config block, either as a raw
// text/plain body (the whole body is the config block) or as JSON
// (`{"config": "..."}`, selected via Content-Type: application/json).
func (h *Handler) remote(w http.ResponseWriter, r *http.Request) {
	if !h.enabledFn() {
		writeJSONError(w, http.StatusConflict, "cloud sync is not enabled")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxRemoteConfigBytes+1))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "failed to read request body")
		return
	}
	if len(body) > maxRemoteConfigBytes {
		writeJSONError(w, http.StatusBadRequest, "remote config exceeds maximum size")
		return
	}

	block := string(body)
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		var payload struct {
			Config string `json:"config"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		block = payload.Config
	}

	if err := h.client.WriteRemoteConfig(r.Context(), block); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getLastReconcileAt() *time.Time {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastReconcileAt
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

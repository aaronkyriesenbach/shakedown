package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"shakedown/internal/auth"
	"shakedown/internal/comments"
	"shakedown/internal/config"
	"shakedown/internal/database"
	apimiddleware "shakedown/internal/middleware"
	"shakedown/internal/recordings"
	"shakedown/internal/songs"
	"shakedown/internal/static"
	"shakedown/internal/tags"
	"shakedown/internal/admin"
	"shakedown/internal/shares"
)

var (
    version = "dev"
    commit  = "unknown"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = logger.Sync() }()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	ctx := context.Background()
	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Warn("database connection failed (will retry on requests)", zap.Error(err))
	}
	if db != nil {
		defer db.Close()
	}

	var authHandler *auth.Handler
	var requireAuth func(http.Handler) http.Handler
	if cfg.DisableAuth {
		logger.Warn("authentication disabled via DISABLE_AUTH — all requests use a synthetic dev user")
		requireAuth = auth.DevAuth(db)
	} else {
		requireAuth = auth.RequireAuth(db)
		authProvider, err := auth.NewProvider(ctx, cfg)
		if err != nil {
			logger.Warn("OIDC provider unavailable (auth endpoints disabled)", zap.Error(err))
		} else {
			authHandler = auth.NewHandler(db, authProvider, cfg, logger)
		}
	}

	var recHandler *recordings.Handler
	var recSvc *recordings.Service
	var songHandler *songs.Handler
	var commentHandler *comments.Handler
	var tagHandler *tags.Handler
	var shareHandler *shares.Handler
	var adminHandler *admin.Handler
	if db != nil {
		store, err := recordings.NewLocalStorage(cfg.StorageRoot)
		if err != nil {
			logger.Fatal("failed to initialize storage", zap.Error(err))
		}
		recRepo := recordings.NewRepository(db)
		recSvc = recordings.NewService(recRepo, store, logger, cfg.ProcessingMaxWorkers)
		recHandler = recordings.NewHandler(recSvc, cfg, logger)

		songRepo := songs.NewRepository(db)
		songHandler = songs.NewHandler(songRepo, logger)

		commentRepo := comments.NewRepository(db)
		commentHandler = comments.NewHandler(commentRepo, logger)

		tagRepo := tags.NewRepository(db)
		tagHandler = tags.NewHandler(tagRepo, logger)

        shareRepo := shares.NewRepository(db)
        shareHandler = shares.NewHandler(shareRepo, recRepo, store, logger)

		adminHandler = admin.NewHandler(db, cfg.StorageRoot, logger)
	}

	r := chi.NewRouter()

	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(apimiddleware.Logger(logger))
	r.Use(apimiddleware.SecurityHeaders)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", healthHandler)

		r.Route("/auth", func(r chi.Router) {
			if cfg.DisableAuth {
				r.With(requireAuth).Get("/me", func(w http.ResponseWriter, r *http.Request) {
					user := auth.UserFromContext(r.Context())
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(user)
				})
			} else if authHandler != nil {
				authHandler.Routes(r)
			}
		})
		r.Route("/recordings", func(r chi.Router) {
			if recHandler != nil {
				recHandler.Routes(r, requireAuth, songHandler, commentHandler, tagHandler)
			}
		})
		r.With(requireAuth).Route("/tags", func(r chi.Router) {
			if tagHandler != nil {
				tagHandler.Routes(r)
			}
		})
		r.Route("/shares", func(r chi.Router) {
			if shareHandler != nil {
				r.With(requireAuth).Post("/", shareHandler.CreateShare)
			}
		})
		r.Route("/s/{token}", func(r chi.Router) {
			if shareHandler != nil {
				r.Use(apimiddleware.RateLimit(100, time.Minute))
				r.Use(shares.TokenMiddleware(db))
				r.Get("/", shareHandler.GetShare)
				r.Get("/stream", shareHandler.StreamShare)
				r.Get("/waveform", shareHandler.WaveformShare)
				r.Get("/download", shareHandler.DownloadShare)
			}
		})
		r.With(requireAuth).Route("/admin", func(r chi.Router) {
			if adminHandler != nil {
				adminHandler.Routes(r)
			}
		})
	})

	r.Get("/*", static.SPAHandler().ServeHTTP)

	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	logger.Info("starting shakedown server", zap.String("addr", addr))

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if recSvc != nil {
		recSvc.Shutdown()
	}

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}

	logger.Info("server stopped")
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(map[string]string{
        "status":  "ok",
        "version": version,
    })
}

package web

import (
	"net/http"
	"time"

	"github.com/jaysongiroux/go-scheduler/internal/config"
	"github.com/jaysongiroux/go-scheduler/internal/logger"
)

func NewWebServer(cfg *config.Config, handler http.Handler) *http.Server {
	server := &http.Server{
		Addr:         cfg.WebServerAddress,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("HTTP server starting on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed: %v", err)
		}
	}()

	return server
}

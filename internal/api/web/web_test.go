package web

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/jaysongiroux/go-scheduler/internal/config"
)

func TestNewWebServer_ConfiguresServerCorrectly(t *testing.T) {
	handler := http.NewServeMux()

	cfg := &config.Config{
		// Use port 0 to let the OS pick a free port
		WebServerAddress: "127.0.0.1:0",
	}

	server := NewWebServer(cfg, handler)

	if server == nil {
		t.Fatal("expected server, got nil")
	}

	// Address
	if server.Addr != cfg.WebServerAddress {
		t.Fatalf("expected addr %s, got %s", cfg.WebServerAddress, server.Addr)
	}

	// Handler
	if server.Handler != handler {
		t.Fatal("expected handler to be assigned")
	}

	// Timeouts
	if server.ReadTimeout != 15*time.Second {
		t.Fatalf("expected ReadTimeout 15s, got %v", server.ReadTimeout)
	}

	if server.WriteTimeout != 15*time.Second {
		t.Fatalf("expected WriteTimeout 15s, got %v", server.WriteTimeout)
	}

	if server.IdleTimeout != 60*time.Second {
		t.Fatalf("expected IdleTimeout 60s, got %v", server.IdleTimeout)
	}

	// Shutdown proves server goroutine started safely
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("server shutdown failed: %v", err)
	}
}

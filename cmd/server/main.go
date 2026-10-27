package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/geiserx/duplicacy-mcp/client"
	"github.com/geiserx/duplicacy-mcp/config"
	"github.com/geiserx/duplicacy-mcp/version"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	log.Printf("Duplicacy MCP %s starting…", version.String())

	cfg := config.LoadDuplicacyConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatal(err)
	}
	s := newServer(client.New(cfg.ExporterURL))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.Transport == "stdio" {
		log.Println("Duplicacy MCP bridge running on stdio")
		if err := server.NewStdioServer(s).Listen(ctx, os.Stdin, os.Stdout); err != nil && !errors.Is(err, context.Canceled) {
			log.Fatalf("stdio server error: %v", err)
		}
		return
	}

	httpCfg, err := config.LoadHTTPConfig()
	if err != nil {
		log.Fatal(err)
	}
	h, err := newHTTPHandler(s, httpCfg)
	if err != nil {
		log.Fatal(err)
	}
	srv := newHTTPServer(httpCfg.ListenAddr, h)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	auth := "bearer token required"
	if httpCfg.AuthToken == "" {
		auth = "no authentication"
	}
	log.Printf("Duplicacy MCP bridge listening on %s%s (%s)", httpCfg.ListenAddr, endpointPath, auth)

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}
}

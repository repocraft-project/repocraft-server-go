package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/repocraft-project/repocraft-server-go/internal/infra/git/httpsmart"
)

// githttpd launches a Smart HTTP server on :8080.
// Repositories are served from ./repositories by default.
func main() {
	rootAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		exitErr(fmt.Sprintf("resolve repo root: %v", err))
	}

	if err := os.MkdirAll(rootAbs, 0o755); err != nil {
		exitErr(fmt.Sprintf("create repo root: %v", err))
	}

	handler := &httpsmart.Server{
		RepoRoot:        rootAbs,
		UploadPackPath:  uploadPackPath,
		ReceivePackPath: receivePackPath,
	}

	server := &http.Server{
		Addr:         httpListenAddr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	fmt.Printf("Serving Git Smart HTTP on %s (repos under %s)\n", httpListenAddr, rootAbs)

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			exitErr(err.Error())
		}
	case sig := <-sigCh:
		fmt.Printf("Received signal %s, shutting down...\n", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			exitErr(fmt.Sprintf("shutdown error: %v", err))
		}
	}
}

func exitErr(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}

const (
	repoRoot        = "./repositories"
	httpListenAddr  = ":8080"
	uploadPackPath  = ""
	receivePackPath = ""
)

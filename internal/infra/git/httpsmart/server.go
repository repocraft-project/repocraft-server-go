package httpsmart

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/repocraft-project/repocraft-server-go/internal/infra/git/service"
)

// Server implements a minimal Git Smart HTTP server backed by git-upload-pack and git-receive-pack.
// It handles:
//   - GET  /<repo>/info/refs?service=git-upload-pack|git-receive-pack (advertise refs)
//   - POST /<repo>/git-upload-pack
//   - POST /<repo>/git-receive-pack
type Server struct {
	RepoRoot        string
	UploadPackPath  string
	ReceivePackPath string
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasSuffix(r.URL.Path, "/info/refs"):
		s.handleInfoRefs(w, r)
	case strings.HasSuffix(r.URL.Path, "/git-upload-pack"):
		s.handleServiceRPC(w, r, service.ServiceUploadPack)
	case strings.HasSuffix(r.URL.Path, "/git-receive-pack"):
		s.handleServiceRPC(w, r, service.ServiceReceivePack)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleInfoRefs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	serviceName := r.URL.Query().Get("service")
	svc, err := parseServiceParam(serviceName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	repoPath, err := s.repoPathFromURL(strings.TrimSuffix(r.URL.Path, "/info/refs"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	contentType := fmt.Sprintf("application/x-%s-advertisement", svc.Command())
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")

	// Write service header pkt-line then flush.
	headerLine := fmt.Sprintf("# service=%s\n", svc.Command())
	if _, err := fmt.Fprintf(w, "%04x%s", len(headerLine)+4, headerLine); err != nil {
		return
	}
	if _, err := io.WriteString(w, "0000"); err != nil {
		return
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	if err := s.runStatelessRPC(r.Context(), w, svc, repoPath, true, nil); err != nil {
		log.Printf("info/refs %s: %v", repoPath, err)
	}
}

func (s *Server) handleServiceRPC(w http.ResponseWriter, r *http.Request, svc service.Service) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	repoPath, err := s.repoPathFromURL(strings.TrimSuffix(r.URL.Path, "/"+svc.Command()))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var contentType string
	switch svc {
	case service.ServiceUploadPack:
		contentType = "application/x-git-upload-pack-result"
	case service.ServiceReceivePack:
		contentType = "application/x-git-receive-pack-result"
	default:
		http.Error(w, "unsupported service", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache")

	if err := s.runStatelessRPC(r.Context(), w, svc, repoPath, false, r.Body); err != nil {
		log.Printf("%s %s: %v", svc.Command(), repoPath, err)
	}
}

func (s *Server) runStatelessRPC(ctx context.Context, stdout io.Writer, svc service.Service, repoPath string, advertise bool, stdin io.Reader) error {
	if !svc.IsSupported() {
		return fmt.Errorf("unsupported service: %s", svc)
	}

	repoFull := filepath.Join(s.RepoRoot, filepath.FromSlash(strings.TrimPrefix(repoPath, "/")))
	if err := ensureWithinRoot(s.RepoRoot, repoFull); err != nil {
		return err
	}
	if _, err := os.Stat(repoFull); err != nil {
		return fmt.Errorf("repo not found: %w", err)
	}

	args := []string{"--stateless-rpc"}
	if advertise {
		args = append(args, "--advertise-refs")
	}
	args = append(args, repoFull)

	binary := svc.Command()
	if svc == service.ServiceUploadPack && s.UploadPackPath != "" {
		binary = s.UploadPackPath
	}
	if svc == service.ServiceReceivePack && s.ReceivePackPath != "" {
		binary = s.ReceivePackPath
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (s *Server) repoPathFromURL(prefix string) (string, error) {
	cleaned := pathClean(prefix)
	if cleaned == "" || cleaned == "/" {
		return "", errors.New("invalid repository path")
	}
	return cleaned, nil
}

func parseServiceParam(raw string) (service.Service, error) {
	switch raw {
	case "git-upload-pack":
		return service.ServiceUploadPack, nil
	case "git-receive-pack":
		return service.ServiceReceivePack, nil
	default:
		return "", fmt.Errorf("unsupported service %q", raw)
	}
}

func ensureWithinRoot(root, full string) error {
	rootClean := filepath.Clean(root)
	fullClean := filepath.Clean(full)
	if rootClean == fullClean {
		return nil
	}
	if !strings.HasPrefix(fullClean, rootClean+string(os.PathSeparator)) {
		return errors.New("invalid path traversal")
	}
	return nil
}

func pathClean(p string) string {
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")
	p = filepath.ToSlash(filepath.Clean(p))
	if p == "." {
		return "/"
	}
	return "/" + p
}

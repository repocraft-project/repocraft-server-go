package ssh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	gossh "github.com/gliderlabs/ssh"
	xssh "golang.org/x/crypto/ssh"

	"github.com/repocraft-project/repocraft-server-go/internal/infra/git/service"
)

// Server exposes a minimal SSH endpoint that only accepts git-upload-pack and git-receive-pack.
// Public key authentication is enforced via an authorized_keys file.
type Server struct {
	Addr               string
	RepoRoot           string
	HostKeyPath        string
	AuthorizedKeysPath string
	UploadPackPath     string
	ReceivePackPath    string
	BaseEnv            []string
	GracefulTimeout    time.Duration
}

// ListenAndServe starts the SSH server and blocks until the context is cancelled or the server stops.
func (s *Server) ListenAndServe(ctx context.Context) error {
	if s.RepoRoot == "" {
		return errors.New("missing repository root")
	}
	if s.HostKeyPath == "" {
		return errors.New("missing SSH host key path")
	}
	if s.AuthorizedKeysPath == "" {
		return errors.New("missing authorized_keys path")
	}

	rootAbs, err := filepath.Abs(s.RepoRoot)
	if err != nil {
		return fmt.Errorf("resolve repo root: %w", err)
	}
	if err := os.MkdirAll(rootAbs, 0o755); err != nil {
		return fmt.Errorf("ensure repo root: %w", err)
	}

	authorized, err := loadAuthorizedKeys(s.AuthorizedKeysPath)
	if err != nil {
		return fmt.Errorf("load authorized keys: %w", err)
	}

	server := &gossh.Server{
		Addr:             s.Addr,
		Handler:          s.handleSession,
		PublicKeyHandler: authorizeKey(authorized),
	}
	server.SetOption(gossh.HostKeyFile(s.HostKeyPath))

	active := &sessionCounter{}
	server.Handler = wrapSessionCount(server.Handler, active)

	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		_ = server.Close() // stop accepting new connections
		waitCtx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout())
		defer cancel()
		active.wait(waitCtx)
		<-errCh // ignore shutdown error when context is cancelled
		wg.Wait()
		return nil
	case err := <-errCh:
		wg.Wait()
		return err
	}
}

func (s *Server) handleSession(sess gossh.Session) {
	rawCmd := sess.RawCommand()
	req, err := service.ParseSSHCommand(rawCmd)
	if err != nil {
		fmt.Fprintf(sess.Stderr(), "invalid command: %v\n", err)
		_ = sess.Exit(1)
		return
	}

	repoFull, err := s.resolveRepoPath(req.RepoPath)
	if err != nil {
		fmt.Fprintf(sess.Stderr(), "invalid repo path: %v\n", err)
		_ = sess.Exit(1)
		return
	}
	if _, err := os.Stat(repoFull); err != nil {
		fmt.Fprintf(sess.Stderr(), "repository not found: %v\n", err)
		_ = sess.Exit(1)
		return
	}

	exec := service.ServiceExecutor{
		UploadPackPath:  s.UploadPackPath,
		ReceivePackPath: s.ReceivePackPath,
		BaseEnv:         s.BaseEnv,
	}
	execReq := service.ServiceRequest{
		Service:         req.Service,
		RepoPath:        repoFull,
		ProtocolVersion: gitProtocolEnv(sess.Environ()),
	}

	if err := exec.Serve(sess.Context(), execReq, sess, sess, sess.Stderr()); err != nil {
		fmt.Fprintf(sess.Stderr(), "git service failed: %v\n", err)
		_ = sess.Exit(1)
		return
	}

	_ = sess.Exit(0)
}

func (s *Server) resolveRepoPath(raw string) (string, error) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.Trim(cleaned, "\"'")
	cleaned = filepath.ToSlash(filepath.Clean(cleaned))
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "" || cleaned == "." {
		return "", errors.New("empty path")
	}
	full := filepath.Join(s.RepoRoot, filepath.FromSlash(cleaned))
	if err := ensureWithinRoot(s.RepoRoot, full); err != nil {
		return "", err
	}
	return full, nil
}

func gitProtocolEnv(env []string) string {
	for _, e := range env {
		if strings.HasPrefix(e, "GIT_PROTOCOL=") {
			return strings.TrimPrefix(e, "GIT_PROTOCOL=")
		}
	}
	return ""
}

func authorizeKey(authorized [][]byte) gossh.PublicKeyHandler {
	return func(ctx gossh.Context, key gossh.PublicKey) bool {
		marshaled := key.Marshal()
		for _, allowed := range authorized {
			if bytes.Equal(marshaled, allowed) {
				return true
			}
		}
		return false
	}
}

func loadAuthorizedKeys(path string) ([][]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var keys [][]byte
	for len(data) > 0 {
		pubKey, _, _, rest, err := xssh.ParseAuthorizedKey(data)
		if err != nil {
			return nil, err
		}
		keys = append(keys, pubKey.Marshal())
		data = rest
	}
	if len(keys) == 0 {
		return nil, errors.New("no authorized keys configured")
	}
	return keys, nil
}

func ensureWithinRoot(root, full string) error {
	rootClean := filepath.Clean(root)
	fullClean := filepath.Clean(full)
	if rootClean == fullClean {
		return nil
	}
	if !strings.HasPrefix(fullClean, rootClean+string(os.PathSeparator)) {
		return errors.New("path traversal detected")
	}
	return nil
}

func (s *Server) shutdownTimeout() time.Duration {
	if s.GracefulTimeout > 0 {
		return s.GracefulTimeout
	}
	return 10 * time.Second
}

type sessionCounter struct {
	mu     sync.Mutex
	active int
	cond   *sync.Cond
}

func (c *sessionCounter) add(delta int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.active += delta
	if c.cond != nil {
		c.cond.Broadcast()
	}
}

func (c *sessionCounter) wait(ctx context.Context) {
	c.mu.Lock()
	if c.cond == nil {
		c.cond = sync.NewCond(&c.mu)
	}
	for c.active > 0 {
		select {
		case <-ctx.Done():
			c.mu.Unlock()
			return
		default:
		}
		c.cond.Wait()
		if ctx.Err() != nil {
			c.mu.Unlock()
			return
		}
	}
	c.mu.Unlock()
}

func wrapSessionCount(next gossh.Handler, counter *sessionCounter) gossh.Handler {
	return func(s gossh.Session) {
		if counter != nil {
			counter.add(1)
			defer counter.add(-1)
		}
		next(s)
	}
}

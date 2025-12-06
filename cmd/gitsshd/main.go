package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	gitssh "github.com/repocraft-project/repocraft-server-go/internal/infra/git/ssh"
)

const (
	listenAddr         = ":2222"
	repoRoot           = "./.repositories"
	sshDir             = "./.ssh"
	hostKeyPath        = "./.ssh/hostkey"
	authorizedKeysPath = "./.ssh/authorized_keys"
	uploadPackPath     = ""
	receivePackPath    = ""
)

// gitsshd launches an SSH server demo that only serves git-upload-pack and git-receive-pack.
func main() {
	if err := setupDemo(); err != nil {
		fmt.Fprintf(os.Stderr, "setup error: %v\n", err)
		os.Exit(1)
	}

	server := gitssh.Server{
		Addr:               listenAddr,
		RepoRoot:           repoRoot,
		HostKeyPath:        hostKeyPath,
		AuthorizedKeysPath: authorizedKeysPath,
		UploadPackPath:     uploadPackPath,
		ReceivePackPath:    receivePackPath,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fmt.Printf("Serving Git SSH on %s (repos under %s)\n", listenAddr, repoRoot)
	fmt.Printf("Add public keys to %s and place bare repos under %s\n", authorizedKeysPath, repoRoot)
	fmt.Printf("Example: git clone ssh://localhost%s/owner/repo.git\n", listenAddr)
	if err := server.ListenAndServe(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ssh server error: %v\n", err)
		os.Exit(1)
	}
}

func setupDemo() error {
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		return fmt.Errorf("create repo root: %w", err)
	}
	if err := os.MkdirAll(sshDir, 0o755); err != nil {
		return fmt.Errorf("create ssh dir: %w", err)
	}

	if err := ensureKey(hostKeyPath, "repocraft-demo-host"); err != nil {
		return fmt.Errorf("ensure host key: %w", err)
	}
	if err := ensureAuthorizedFile(authorizedKeysPath); err != nil {
		return fmt.Errorf("ensure authorized_keys: %w", err)
	}
	return nil
}

func ensureKey(path, comment string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", path, "-N", "", "-C", comment)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ensureAuthorizedFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, []byte{}, 0o600)
}

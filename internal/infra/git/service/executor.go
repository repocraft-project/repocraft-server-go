package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// ServiceExecutor executes git service binaries (upload-pack/receive-pack).
type ServiceExecutor struct {
	// UploadPackPath and ReceivePackPath optionally override the binary names.
	// If left empty, defaults (git-upload-pack/git-receive-pack) are used.
	UploadPackPath  string
	ReceivePackPath string
	// BaseEnv is appended to the inherited environment before invoking git.
	BaseEnv []string
	// WorkDir optionally sets the working directory for spawned commands.
	WorkDir string
}

// Serve runs the git service for the given request, streaming I/O.
func (e ServiceExecutor) Serve(ctx context.Context, req ServiceRequest, stdin io.Reader, stdout, stderr io.Writer) error {
	if err := req.Validate(); err != nil {
		return err
	}

	binary, err := e.resolveBinary(req.Service)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, binary, req.RepoPath)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Dir = e.WorkDir
	cmd.Env = append(os.Environ(), e.BaseEnv...)
	if req.ProtocolVersion != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_PROTOCOL=%s", req.ProtocolVersion))
	}

	return cmd.Run()
}

func (e ServiceExecutor) resolveBinary(service Service) (string, error) {
	switch service {
	case ServiceUploadPack:
		if e.UploadPackPath != "" {
			return e.UploadPackPath, nil
		}
		return ServiceUploadPack.Command(), nil
	case ServiceReceivePack:
		if e.ReceivePackPath != "" {
			return e.ReceivePackPath, nil
		}
		return ServiceReceivePack.Command(), nil
	default:
		return "", fmt.Errorf("unsupported service: %s", service)
	}
}

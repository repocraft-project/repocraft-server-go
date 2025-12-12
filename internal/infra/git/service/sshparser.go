package service

import (
	"fmt"
	"strings"
)

// ParseSSHCommand parses an SSH command string like:
//
//	git-upload-pack '/var/repos/foo.git'
//
// Returns a ServiceRequest with service and repo path extracted.
func ParseSSHCommand(command string) (ServiceRequest, error) {
	raw := strings.TrimSpace(command)
	if raw == "" {
		return ServiceRequest{}, fmt.Errorf("empty command")
	}

	parts := strings.Fields(raw)
	if len(parts) < 2 {
		return ServiceRequest{}, fmt.Errorf("invalid command: %s", raw)
	}

	service := Service(parts[0])
	repoToken := parts[1]
	repoToken = strings.Trim(repoToken, "\"'")

	req := ServiceRequest{
		Service:  service,
		RepoPath: repoToken,
	}

	if err := req.Validate(); err != nil {
		return ServiceRequest{}, err
	}

	return req, nil
}

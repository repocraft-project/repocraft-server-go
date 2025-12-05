package service

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// Transport enumerates Git transport protocols as described in the Git Book.
type Transport string

const (
	TransportLocal Transport = "local"
	TransportSSH   Transport = "ssh"
	TransportGit   Transport = "git"
	TransportHTTP  Transport = "http"
	TransportHTTPS Transport = "https"
)

// Endpoint represents a Git repository location including transport details.
type Endpoint struct {
	Transport Transport
	User      string
	Host      string
	Port      string
	Path      string
}

// ParseEndpoint parses a Git URL or scp-like syntax into an Endpoint.
// Examples:
//
//	/srv/git/project.git
//	git@example.com:repo/project.git
//	ssh://git@example.com:2222/repo/project.git
//	https://example.com/repo/project.git
func ParseEndpoint(raw string) (Endpoint, error) {
	if raw == "" {
		return Endpoint{}, fmt.Errorf("empty endpoint")
	}

	// Local path shortcut: no scheme, no host separator.
	if !strings.Contains(raw, "://") && !strings.Contains(raw, "@") && !strings.Contains(raw, ":") {
		return Endpoint{
			Transport: TransportLocal,
			Path:      raw,
		}, nil
	}

	// scp-like syntax: [user@]host:path
	if !strings.Contains(raw, "://") && strings.Contains(raw, ":") {
		userHost, repoPath, found := strings.Cut(raw, ":")
		if !found {
			return Endpoint{}, fmt.Errorf("invalid scp-like syntax")
		}
		user, host := splitUserHost(userHost)
		return Endpoint{
			Transport: TransportSSH,
			User:      user,
			Host:      host,
			Path:      ensureLeadingSlash(repoPath),
		}, nil
	}

	u, err := url.Parse(raw)
	if err != nil {
		return Endpoint{}, err
	}

	var transport Transport
	switch strings.ToLower(u.Scheme) {
	case "ssh":
		transport = TransportSSH
	case "git":
		transport = TransportGit
	case "http":
		transport = TransportHTTP
	case "https":
		transport = TransportHTTPS
	default:
		return Endpoint{}, fmt.Errorf("unsupported transport: %s", u.Scheme)
	}

	return Endpoint{
		Transport: transport,
		User:      u.User.Username(),
		Host:      u.Hostname(),
		Port:      u.Port(),
		Path:      ensureLeadingSlash(path.Clean(u.Path)),
	}, nil
}

func splitUserHost(s string) (user, host string) {
	if strings.Contains(s, "@") {
		parts := strings.SplitN(s, "@", 2)
		return parts[0], parts[1]
	}
	return "", s
}

func ensureLeadingSlash(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		return "/" + p
	}
	return p
}

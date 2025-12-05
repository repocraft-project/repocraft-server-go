package service

import "fmt"

// ServiceRequest describes an incoming SSH git service request.
type ServiceRequest struct {
	Service         Service
	RepoPath        string
	ProtocolVersion string // e.g. "version=2" when Git wants protocol v2
}

// Validate performs a basic sanity check on the request.
func (r ServiceRequest) Validate() error {
	if !r.Service.IsSupported() {
		return fmt.Errorf("unsupported service: %s", r.Service)
	}
	if r.RepoPath == "" {
		return fmt.Errorf("missing repository path")
	}
	return nil
}

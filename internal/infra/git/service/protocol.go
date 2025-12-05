package service

// Service represents the Git service invoked by the client.
// These are the well-known services described in the Git wire protocol.
type Service string

const (
	ServiceUploadPack  Service = "git-upload-pack"
	ServiceReceivePack Service = "git-receive-pack"
)

// Command returns the executable name associated with the service.
func (s Service) Command() string {
	return string(s)
}

// IsSupported reports whether the service is recognized.
func (s Service) IsSupported() bool {
	return s == ServiceUploadPack || s == ServiceReceivePack
}

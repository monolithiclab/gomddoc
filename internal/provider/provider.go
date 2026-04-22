package provider

// Provider defines the interface for content providers
type Provider interface {
	// ReadFile reads a file at the given path
	ReadFile(path string) ([]byte, error)
	// DefaultIndex returns the default index file name
	DefaultIndex() string
}

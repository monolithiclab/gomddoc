package processor

// Processor defines the interface for document processors
type Processor interface {
	// Process converts content from one format to another
	Process(content []byte) ([]byte, error)
	// ContentType returns the MIME type of the processed content
	ContentType() string
}

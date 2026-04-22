package enricher

import (
	"context"
	"testing"
)

func TestNoOpEnricher_SupportedMimeTypes(t *testing.T) {
	t.Parallel()
	e := &NoOpEnricher{}
	types := e.SupportedMimeTypes()
	if types != nil {
		t.Errorf("SupportedMimeTypes() = %v, want nil", types)
	}
}

func TestNoOpEnricher_Enrich(t *testing.T) {
	t.Parallel()
	e := &NoOpEnricher{}

	result, err := e.Enrich(context.Background(), []byte("some content"), "/test.txt")
	if err != nil {
		t.Fatalf("Enrich() error = %v", err)
	}

	if result == nil {
		t.Fatal("Enrich() returned nil")
	}

	if result.Metadata != nil {
		t.Errorf("Metadata = %v, want nil", result.Metadata)
	}
	if result.TOC != nil {
		t.Errorf("TOC = %v, want nil", result.TOC)
	}
	if result.Navigation != nil {
		t.Errorf("Navigation = %v, want nil", result.Navigation)
	}
	if result.RelatedDocs != nil {
		t.Errorf("RelatedDocs = %v, want nil", result.RelatedDocs)
	}
}

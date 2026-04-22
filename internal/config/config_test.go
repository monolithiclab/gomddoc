package config

import (
	"os"
	"testing"
)

func TestNew(t *testing.T) {
	config := New()

	if config.Dir != "." {
		t.Errorf("Expected default dir '.', got %q", config.Dir)
	}

	if config.Port != ":8080" {
		t.Errorf("Expected default port ':8080', got %q", config.Port)
	}

	if config.DefaultIndex != "README.md" {
		t.Errorf("Expected default index 'README.md', got %q", config.DefaultIndex)
	}

	if config.ShutdownTimeout.Seconds() != 1 {
		t.Errorf("Expected shutdown timeout 1s, got %v", config.ShutdownTimeout)
	}
}

func TestParseFlags(t *testing.T) {
	// Save original args to restore later
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Test default values
	os.Args = []string{"cmd"}
	config := New()
	config.ParseFlags()

	if config.Dir != "." {
		t.Errorf("Expected default dir '.', got %q", config.Dir)
	}

	if config.Port != ":8080" {
		t.Errorf("Expected default port ':8080', got %q", config.Port)
	}
}

func TestValidate(t *testing.T) {
	config := New()
	err := config.Validate()
	if err != nil {
		t.Errorf("Expected validation to pass, got error: %v", err)
	}
}

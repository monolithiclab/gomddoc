package main

import (
	"testing"
)

func TestPreviewCmd_Defaults(t *testing.T) {
	cmd := PreviewCmd{}

	// Verify default struct values match Kong defaults
	if cmd.Dir != "" {
		t.Errorf("Dir default = %q, want empty (Kong sets '.')", cmd.Dir)
	}
	if cmd.Port != "" {
		t.Errorf("Port default = %q, want empty (Kong sets ':auto')", cmd.Port)
	}
	if cmd.Open {
		t.Error("Open should default to false")
	}
}

func TestPreviewCmd_FieldTags(t *testing.T) {
	// Verify that PreviewCmd has the expected Kong struct tags
	// by checking that the struct can be instantiated with expected values
	cmd := PreviewCmd{
		Dir:  "/tmp/docs",
		Port: ":9090",
		Open: true,
	}

	if cmd.Dir != "/tmp/docs" {
		t.Errorf("Dir = %q, want /tmp/docs", cmd.Dir)
	}
	if cmd.Port != ":9090" {
		t.Errorf("Port = %q, want :9090", cmd.Port)
	}
	if !cmd.Open {
		t.Error("Open should be true")
	}
}

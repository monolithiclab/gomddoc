package main

import (
	"bytes"
	"testing"

	"github.com/monolithiclab/gomddoc/internal/config"
)

func TestSchemaCmd(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := (&SchemaCmd{out: &buf}).Run(); err != nil {
		t.Fatal(err)
	}
	if buf.String() != string(config.JSONSchema())+"\n" {
		t.Error("schema output differs from config.JSONSchema()")
	}
}

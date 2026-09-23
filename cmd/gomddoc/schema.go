package main

import (
	"fmt"
	"io"
	"os"

	"github.com/monolithiclab/gomddoc/internal/config"
)

// SchemaCmd prints the JSON Schema for .gomddoc/config.yml.
type SchemaCmd struct {
	out io.Writer // nil: os.Stdout
}

// Run writes config.JSONSchema.
func (s *SchemaCmd) Run() error {
	w := s.out
	if w == nil {
		w = os.Stdout
	}
	_, err := fmt.Fprintf(w, "%s\n", config.JSONSchema())
	return err
}

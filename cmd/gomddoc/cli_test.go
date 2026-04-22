package main

import (
	"reflect"
	"testing"
)

func TestServeCmd_EnvTags(t *testing.T) {
	t.Parallel()
	assertEnvTag[ServeCmd](t, "Dir", "GOMDDOC_SERVER_DIR")
	assertEnvTag[ServeCmd](t, "Port", "GOMDDOC_SERVER_PORT")
	assertEnvTag[ServeCmd](t, "Domain", "GOMDDOC_DOMAIN")
	assertEnvTag[ServeCmd](t, "GitSSHKey", "GOMDDOC_SERVER_GIT_SSH_KEY")
	assertEnvTag[ServeCmd](t, "GitStorageDir", "GOMDDOC_SERVER_GIT_STORAGE_DIR")
	assertEnvTag[ServeCmd](t, "Pprof", "GOMDDOC_SERVER_PPROF")
}

func TestBuildCmd_EnvTags(t *testing.T) {
	t.Parallel()
	assertEnvTag[BuildCmd](t, "Dir", "GOMDDOC_SERVER_DIR")
	assertEnvTag[BuildCmd](t, "Output", "GOMDDOC_BUILD_OUTPUT")
	assertEnvTag[BuildCmd](t, "GitSSHKey", "GOMDDOC_SERVER_GIT_SSH_KEY")
	assertEnvTag[BuildCmd](t, "GitStorageDir", "GOMDDOC_SERVER_GIT_STORAGE_DIR")
}

func TestPreviewCmd_EnvTags(t *testing.T) {
	t.Parallel()
	assertEnvTag[PreviewCmd](t, "Dir", "GOMDDOC_SERVER_DIR")
	assertEnvTag[PreviewCmd](t, "Port", "GOMDDOC_SERVER_PORT")
	assertEnvTag[PreviewCmd](t, "Open", "GOMDDOC_PREVIEW_OPEN")
	assertEnvTag[PreviewCmd](t, "GitSSHKey", "GOMDDOC_SERVER_GIT_SSH_KEY")
	assertEnvTag[PreviewCmd](t, "GitStorageDir", "GOMDDOC_SERVER_GIT_STORAGE_DIR")
}

func TestMCPCmd_EnvTags(t *testing.T) {
	t.Parallel()
	assertEnvTag[MCPCmd](t, "Dir", "GOMDDOC_SERVER_DIR")
	assertEnvTag[MCPCmd](t, "GitSSHKey", "GOMDDOC_SERVER_GIT_SSH_KEY")
	assertEnvTag[MCPCmd](t, "GitStorageDir", "GOMDDOC_SERVER_GIT_STORAGE_DIR")
}

// assertEnvTag verifies that a struct field has the expected Kong env tag.
func assertEnvTag[T any](t *testing.T, fieldName, wantEnv string) {
	t.Helper()
	field, ok := reflect.TypeFor[T]().FieldByName(fieldName)
	if !ok {
		t.Fatalf("field %s not found on %T", fieldName, *new(T))
	}
	got := field.Tag.Get("env")
	if got != wantEnv {
		t.Errorf("%T.%s env tag = %q, want %q", *new(T), fieldName, got, wantEnv)
	}
}

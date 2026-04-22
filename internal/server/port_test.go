package server

import (
	"net"
	"strconv"
	"strings"
	"testing"
)

func TestFindAvailablePort(t *testing.T) {
	port, err := FindAvailablePort(8080)
	if err != nil {
		t.Fatalf("FindAvailablePort(8080) returned error: %v", err)
	}

	if !strings.HasPrefix(port, ":") {
		t.Errorf("expected port to start with ':', got %q", port)
	}

	portNum, err := strconv.Atoi(strings.TrimPrefix(port, ":"))
	if err != nil {
		t.Fatalf("invalid port number in %q: %v", port, err)
	}

	if portNum < 8080 || portNum >= 8080+maxPortScanRange {
		t.Errorf("port %d outside expected range [8080, %d)", portNum, 8080+maxPortScanRange)
	}
}

func TestFindAvailablePort_SkipsOccupied(t *testing.T) {
	// Occupy a port
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	_, occupiedPortStr, _ := net.SplitHostPort(ln.Addr().String())
	occupiedPort, _ := strconv.Atoi(occupiedPortStr)

	// Ask for the occupied port — should get the next one
	port, err := FindAvailablePort(occupiedPort)
	if err != nil {
		t.Fatalf("FindAvailablePort(%d) returned error: %v", occupiedPort, err)
	}

	gotPort, _ := strconv.Atoi(strings.TrimPrefix(port, ":"))
	if gotPort == occupiedPort {
		t.Errorf("expected to skip occupied port %d, but got it", occupiedPort)
	}
	if gotPort <= occupiedPort || gotPort >= occupiedPort+maxPortScanRange {
		t.Errorf("port %d outside expected range (%d, %d)", gotPort, occupiedPort, occupiedPort+maxPortScanRange)
	}
}

func TestIsAutoPort(t *testing.T) {
	tests := []struct {
		port string
		want bool
	}{
		{":auto", true},
		{":0", true},
		{":8080", false},
		{"auto", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsAutoPort(tt.port); got != tt.want {
			t.Errorf("IsAutoPort(%q) = %v, want %v", tt.port, got, tt.want)
		}
	}
}

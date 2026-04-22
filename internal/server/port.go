package server

import (
	"fmt"
	"net"
	"strconv"
)

// AutoPort is the special port value that triggers automatic port discovery.
const AutoPort = ":auto"

// maxPortScanRange is the maximum number of ports to scan from the start port.
const maxPortScanRange = 100

// FindAvailablePort scans for an available TCP port starting from startPort.
// It tries ports sequentially from startPort up to startPort+maxPortScanRange.
// Returns the port in ":PORT" format (e.g., ":8080").
func FindAvailablePort(startPort int) (string, error) {
	for port := startPort; port < startPort+maxPortScanRange; port++ {
		addr := ":" + strconv.Itoa(port)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			continue
		}
		_ = ln.Close()
		return addr, nil
	}
	return "", fmt.Errorf("no available port found in range %d-%d", startPort, startPort+maxPortScanRange-1)
}

// IsAutoPort returns true if the port string indicates automatic port assignment.
func IsAutoPort(port string) bool {
	return port == AutoPort || port == ":0"
}

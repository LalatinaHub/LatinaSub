package netutil

import (
	"fmt"
	"net"
	"sync"
	"time"
)

var (
	portMu         sync.Mutex
	allocatedPorts = make(map[int]time.Time)
	portTtl        = 30 * time.Second
)

// cleanExpiredPortsLocked removes allocated ports that have exceeded their TTL.
// Must be called with portMu held.
func cleanExpiredPortsLocked() {
	now := time.Now()
	for port, ts := range allocatedPorts {
		if now.Sub(ts) > portTtl {
			delete(allocatedPorts, port)
		}
	}
}

// GetFreePort requests an available ephemeral TCP port from the OS and temporarily
// reserves it to minimize port collisions across concurrent workers.
func GetFreePort() (int, error) {
	portMu.Lock()
	defer portMu.Unlock()

	cleanExpiredPortsLocked()

	const maxAttempts = 25
	for attempt := 0; attempt < maxAttempts; attempt++ {
		addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
		if err != nil {
			return 0, fmt.Errorf("failed to resolve tcp addr: %w", err)
		}

		listener, err := net.ListenTCP("tcp", addr)
		if err != nil {
			continue
		}

		tcpAddr, ok := listener.Addr().(*net.TCPAddr)
		_ = listener.Close()
		if !ok {
			continue
		}

		port := tcpAddr.Port
		if _, exists := allocatedPorts[port]; !exists {
			allocatedPorts[port] = time.Now()
			return port, nil
		}
	}

	return 0, fmt.Errorf("failed to allocate free port after %d attempts", maxAttempts)
}

// ReleasePort removes a port reservation from the internal tracker.
func ReleasePort(port int) {
	portMu.Lock()
	defer portMu.Unlock()
	delete(allocatedPorts, port)
}

// AllocatePort finds a free port, reserves it, and returns a release callback.
func AllocatePort() (int, func(), error) {
	port, err := GetFreePort()
	if err != nil {
		return 0, nil, err
	}
	release := func() {
		ReleasePort(port)
	}
	return port, release, nil
}

// MustGetFreePort returns a free port or panics if unable to allocate.
func MustGetFreePort() int {
	port, err := GetFreePort()
	if err != nil {
		panic(err)
	}
	return port
}


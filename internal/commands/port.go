package commands

import (
	"fmt"
	"net"
	"strconv"
)

func assertPortAvailable(host string, port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}

	address := net.JoinHostPort(host, strconv.Itoa(port))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("port %d on host %q is unavailable: %w", port, host, err)
	}

	if err := listener.Close(); err != nil {
		return fmt.Errorf("failed to release port %d on host %q after availability check: %w", port, host, err)
	}
	return nil
}

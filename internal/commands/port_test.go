package commands

import (
	"net"
	"testing"
)

func TestAssertPortAvailable(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on ephemeral port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("close ephemeral listener: %v", err)
	}

	if err := assertPortAvailable("127.0.0.1", port); err != nil {
		t.Errorf("assertPortAvailable() error = %v, want nil", err)
	}
}

func TestAssertPortAvailableBusyPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen on ephemeral port: %v", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Errorf("close listener: %v", err)
		}
	}()

	port := listener.Addr().(*net.TCPAddr).Port
	if err := assertPortAvailable("127.0.0.1", port); err == nil {
		t.Error("assertPortAvailable() error = nil, want busy port error")
	}
}

func TestAssertPortAvailableInvalidPort(t *testing.T) {
	tests := []struct {
		port int
		want string
	}{
		{port: 0, want: "invalid port: 0"},
		{port: 70000, want: "invalid port: 70000"},
	}

	for _, tt := range tests {
		err := assertPortAvailable("127.0.0.1", tt.port)
		if err == nil {
			t.Errorf("assertPortAvailable(%d) error = nil, want %q", tt.port, tt.want)
		} else if err.Error() != tt.want {
			t.Errorf("assertPortAvailable(%d) error = %q, want %q", tt.port, err, tt.want)
		}
	}
}

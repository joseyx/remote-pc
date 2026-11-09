package system

import (
	"net"
	"testing"
)

func TestCheckServices(t *testing.T) {
	// Start a dummy listener
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	targets := []ServiceTarget{
		{ID: "open-svc", Name: "Open Service", Port: port},
		{ID: "closed-svc", Name: "Closed Service", Port: 65432},
	}

	results := CheckServices(targets)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	if !results[0].Running {
		t.Errorf("expected open-svc to be running")
	}
	if results[1].Running {
		t.Errorf("expected closed-svc to not be running")
	}
}

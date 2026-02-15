package handlers

import (
	"net"
	"testing"
)

func TestIPAllowlistSetNormalizesAndAllows(t *testing.T) {
	a, _ := NewIPAllowlist("127.0.0.1,\n::1, 192.168.0.0/16")
	if a.Raw() == "" {
		t.Fatalf("expected raw not empty")
	}
	if !a.AllowsIP(net.ParseIP("127.0.0.1")) {
		t.Fatalf("expected allow 127.0.0.1")
	}
	if !a.AllowsIP(net.ParseIP("192.168.1.10")) {
		t.Fatalf("expected allow 192.168.1.10")
	}
	if a.AllowsIP(net.ParseIP("10.0.0.1")) {
		t.Fatalf("expected deny 10.0.0.1")
	}
}

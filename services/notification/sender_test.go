package notification

import (
	"net"
	"testing"
)

func TestPrivateAddressDetection(t *testing.T) {
	for _, s := range []string{"127.0.0.1", "10.0.0.1", "192.168.1.1", "169.254.1.1", "::1"} {
		if !isPrivateOrLocal(net.ParseIP(s)) {
			t.Fatalf("expected private: %s", s)
		}
	}
	if isPrivateOrLocal(net.ParseIP("8.8.8.8")) {
		t.Fatal("public address classified as private")
	}
}

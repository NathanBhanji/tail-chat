package discovery

import (
	"fmt"
	"testing"
)

func TestGetSelfIP(t *testing.T) {
	ip, host, err := GetSelfIP()
	if err != nil {
		t.Skipf("tailscale not available: %v", err)
	}

	fmt.Printf("Self: %s (%s)\n", host, ip)

	if ip == "" {
		t.Fatal("empty self IP")
	}
	if host == "" {
		t.Fatal("empty hostname")
	}
}

func TestGetPeers(t *testing.T) {
	peers, err := GetPeers()
	if err != nil {
		t.Skipf("tailscale not available: %v", err)
	}

	fmt.Printf("Found %d peers:\n", len(peers))
	for _, p := range peers {
		status := "offline"
		if p.Online {
			status = "online"
		}
		fmt.Printf("  %s %s (%s) [%s]\n", status, p.Hostname, p.TailscaleIP, p.OS)
	}

	if len(peers) == 0 {
		t.Fatal("no peers found")
	}
}

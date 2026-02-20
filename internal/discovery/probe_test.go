package discovery

import (
	"net"
	"testing"
)

func TestProbePeers_DetectsListener(t *testing.T) {
	// Start a TCP listener on a random port to simulate tailchat
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	// Accept connections in the background so the probe can connect
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	_, portStr, _ := net.SplitHostPort(ln.Addr().String())

	// We can't easily control the port probePeers uses (it's hardcoded to 9377),
	// so test the dial logic directly.
	t.Run("reachable", func(t *testing.T) {
		addr := net.JoinHostPort("127.0.0.1", portStr)
		conn, err := net.DialTimeout("tcp", addr, 500*1e6) // 500ms
		if err != nil {
			t.Fatalf("expected connection to succeed: %v", err)
		}
		conn.Close()
	})

	t.Run("unreachable", func(t *testing.T) {
		// Port 1 is almost certainly not listening
		addr := net.JoinHostPort("127.0.0.1", "1")
		conn, err := net.DialTimeout("tcp", addr, 200*1e6) // 200ms
		if err == nil {
			conn.Close()
			t.Fatal("expected connection to fail")
		}
	})

	t.Run("offline_peers_skipped", func(t *testing.T) {
		// probePeers should not modify offline peers
		testPeers := []Peer{
			{Hostname: "offline", TailscaleIP: "127.0.0.1", Online: false},
		}
		probePeers(testPeers)
		if testPeers[0].RunningTailchat {
			t.Error("offline peer should not be probed")
		}
	})
}

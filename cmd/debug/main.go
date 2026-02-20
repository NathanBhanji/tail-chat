package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/NathanBhanji/tail-chat/internal/crypto"
	"github.com/NathanBhanji/tail-chat/internal/discovery"
	tcnet "github.com/NathanBhanji/tail-chat/internal/net"
)

func main() {
	target := "100.66.195.59" // davidmills-macbook-pro
	if len(os.Args) > 1 {
		target = os.Args[1]
	}
	port := tcnet.DefaultPort

	fmt.Println("=== tailchat connection debug ===")
	fmt.Println()

	// 1. Check tailscale
	fmt.Print("1. Tailscale status... ")
	selfIP, selfHost, err := discovery.GetSelfIP()
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("OK (%s @ %s)\n", selfHost, selfIP)

	// 2. Check identity
	fmt.Print("2. Loading identity... ")
	kp, err := crypto.LoadOrGenerateKeyPair()
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("OK (pubkey: %s...)\n", kp.PublicKeyHex()[:16])

	// 3. TCP connectivity
	addr := fmt.Sprintf("%s:%d", target, port)
	fmt.Printf("3. TCP connect to %s... ", addr)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		fmt.Println()
		fmt.Println("   The peer is either:")
		fmt.Println("   - Not running tailchat")
		fmt.Println("   - Firewall blocking port 9377")
		fmt.Println("   - Tailscale connection issue")
		fmt.Println()
		fmt.Println("   Try: ping", target)
		os.Exit(1)
	}
	conn.Close()
	fmt.Println("OK (port open)")

	// 4. Full handshake
	fmt.Printf("4. E2E handshake with %s... ", target)
	peerConn, err := tcnet.Connect(addr, kp, selfHost, nil)
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		fmt.Println()
		fmt.Println("   TCP connected but handshake failed.")
		fmt.Println("   The peer may be running a different version.")
		os.Exit(1)
	}
	fmt.Printf("OK (peer: %s)\n", peerConn.PeerHostname)

	// 5. Shared secret
	fmt.Printf("5. Shared secret derived: %x...%x\n",
		peerConn.SharedSecret[:4], peerConn.SharedSecret[28:])

	// 6. Test encrypt/decrypt roundtrip
	fmt.Print("6. Encrypt/decrypt test... ")
	testMsg := []byte("hello from debug tool")
	ct, err := crypto.Encrypt(peerConn.SharedSecret, testMsg)
	if err != nil {
		fmt.Printf("FAIL encrypt: %v\n", err)
		os.Exit(1)
	}
	pt, err := crypto.Decrypt(peerConn.SharedSecret, ct)
	if err != nil {
		fmt.Printf("FAIL decrypt: %v\n", err)
		os.Exit(1)
	}
	if string(pt) != string(testMsg) {
		fmt.Printf("FAIL mismatch\n")
		os.Exit(1)
	}
	fmt.Println("OK")

	peerConn.Conn.Close()

	fmt.Println()
	fmt.Println("All checks passed! Connection to", target, "is working.")
}

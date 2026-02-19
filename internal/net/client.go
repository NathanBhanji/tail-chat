package net

import (
	"fmt"
	"net"
	"time"

	"github.com/NathanBhanji/tail-chat/internal/crypto"
	"github.com/NathanBhanji/tail-chat/internal/protocol"
)

const (
	DialTimeout = 5 * time.Second
)

// Connect establishes an authenticated, encrypted connection to a peer.
func Connect(addr string, kp *crypto.KeyPair, hostname string) (*Connection, error) {
	conn, err := net.DialTimeout("tcp", addr, DialTimeout)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}

	// Send our handshake
	hs := &protocol.Handshake{
		PublicKey: kp.PublicKey[:],
		Hostname:  hostname,
		Version:   "1.0.0",
	}

	env, err := protocol.Wrap(protocol.TypeHandshake, hs)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("wrap handshake: %w", err)
	}

	if err := protocol.WriteMessage(conn, env); err != nil {
		conn.Close()
		return nil, fmt.Errorf("send handshake: %w", err)
	}

	// Receive peer's handshake
	reply, err := protocol.ReadMessage(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("read handshake reply: %w", err)
	}

	if reply.Type != protocol.TypeHandshake {
		conn.Close()
		return nil, fmt.Errorf("unexpected message type: %d", reply.Type)
	}

	peerHS, err := protocol.Unwrap[protocol.Handshake](reply)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("unwrap handshake: %w", err)
	}

	// Derive shared secret
	var peerPub [32]byte
	copy(peerPub[:], peerHS.PublicKey)

	shared, err := crypto.SharedSecret(kp.PrivateKey, peerPub)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("derive shared secret: %w", err)
	}

	return &Connection{
		Conn:          conn,
		PeerHostname:  peerHS.Hostname,
		PeerPublicKey: peerPub,
		SharedSecret:  shared,
	}, nil
}

// SendEncrypted encrypts and sends a chat message over a connection.
func SendEncrypted(c *Connection, msgType protocol.MessageType, msg any) error {
	env, err := protocol.Wrap(msgType, msg)
	if err != nil {
		return fmt.Errorf("wrap message: %w", err)
	}
	return protocol.WriteMessage(c.Conn, env)
}

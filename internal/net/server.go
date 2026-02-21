package net

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/NathanBhanji/tail-chat/internal/crypto"
	"github.com/NathanBhanji/tail-chat/internal/protocol"
)

const (
	// HandshakeTimeout is the maximum time to complete a handshake.
	HandshakeTimeout = 10 * time.Second
	// ReadTimeout is the deadline for reading a single message.
	ReadTimeout = 5 * time.Minute
	// MaxConnections limits the number of concurrent peer connections.
	MaxConnections = 128
)

// HostnameResolver resolves a Tailscale IP to its authenticated hostname.
// Returns empty string if the IP is not recognized.
type HostnameResolver func(ip string) string

const DefaultPort = 9377

// Connection represents an established, authenticated peer connection.
type Connection struct {
	Conn          net.Conn
	PeerHostname  string
	PeerPublicKey [32]byte
	SharedSecret  [32]byte
	writeMu       sync.Mutex
}

// WriteMessage writes a protocol envelope to the connection with proper serialization.
func (c *Connection) WriteMessage(env *protocol.Envelope) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return protocol.WriteMessage(c.Conn, env)
}

// Server listens for incoming peer connections.
type Server struct {
	listener        net.Listener
	keyPair         *crypto.KeyPair
	knownKeys       *crypto.KnownKeys
	hostname        string
	mu              sync.RWMutex
	conns           map[string]*Connection // hostname -> connection
	onConnect       func(*Connection)
	onMessage       func(*Connection, *protocol.Envelope)
	onDisconnect    func(hostname string)
	onKeyWarning    func(hostname string, err error) // TOFU key change warning
	resolveHostname HostnameResolver
	stopCh          chan struct{}
	stopOnce        sync.Once
}

// NewServer creates a TCP server bound to the given address.
func NewServer(addr string, kp *crypto.KeyPair, hostname string, knownKeys *crypto.KnownKeys) (*Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	return &Server{
		listener:  ln,
		keyPair:   kp,
		knownKeys: knownKeys,
		hostname:  hostname,
		conns:     make(map[string]*Connection),
		stopCh:    make(chan struct{}),
	}, nil
}

// OnConnect sets a callback for new authenticated connections.
func (s *Server) OnConnect(fn func(*Connection)) {
	s.onConnect = fn
}

// OnMessage sets a callback for incoming messages.
func (s *Server) OnMessage(fn func(*Connection, *protocol.Envelope)) {
	s.onMessage = fn
}

// OnDisconnect sets a callback for when a peer disconnects.
func (s *Server) OnDisconnect(fn func(hostname string)) {
	s.onDisconnect = fn
}

// OnKeyWarning sets a callback for TOFU key change warnings.
func (s *Server) OnKeyWarning(fn func(hostname string, err error)) {
	s.onKeyWarning = fn
}

// SetHostnameResolver configures IP-to-hostname resolution for hostname verification.
func (s *Server) SetHostnameResolver(fn HostnameResolver) {
	s.resolveHostname = fn
}

// Addr returns the server's listen address.
func (s *Server) Addr() string {
	return s.listener.Addr().String()
}

// Start begins accepting connections.
func (s *Server) Start() {
	go s.acceptLoop()
}

func (s *Server) acceptLoop() {
	connSem := make(chan struct{}, MaxConnections)
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				log.Printf("accept error: %v", err)
				continue
			}
		}

		// Enforce connection limit
		select {
		case connSem <- struct{}{}:
			go func() {
				defer func() { <-connSem }()
				s.handleConn(conn)
			}()
		default:
			log.Printf("connection limit reached (%d), rejecting %s", MaxConnections, conn.RemoteAddr())
			conn.Close()
		}
	}
}

func (s *Server) handleConn(conn net.Conn) {
	// Enforce handshake timeout
	conn.SetDeadline(time.Now().Add(HandshakeTimeout))

	// Receive peer's handshake
	env, err := protocol.ReadMessage(conn)
	if err != nil {
		conn.Close()
		return
	}

	if env.Type != protocol.TypeHandshake {
		conn.Close()
		return
	}

	hs, err := protocol.Unwrap[protocol.Handshake](env)
	if err != nil {
		conn.Close()
		return
	}

	// Validate public key length
	if len(hs.PublicKey) != 32 {
		conn.Close()
		return
	}

	// Send our handshake back
	ourHS := &protocol.Handshake{
		PublicKey: s.keyPair.PublicKey[:],
		Hostname:  s.hostname,
		Version:   "1.0.0",
	}

	reply, err := protocol.Wrap(protocol.TypeHandshake, ourHS)
	if err != nil {
		conn.Close()
		return
	}

	if err := protocol.WriteMessage(conn, reply); err != nil {
		conn.Close()
		return
	}

	// Clear handshake deadline
	conn.SetDeadline(time.Time{})

	// Derive shared secret
	var peerPub [32]byte
	copy(peerPub[:], hs.PublicKey)

	// TOFU: verify peer's public key against known keys
	if s.knownKeys != nil {
		if err := s.knownKeys.Verify(hs.Hostname, peerPub); err != nil {
			if s.onKeyWarning != nil {
				s.onKeyWarning(hs.Hostname, err)
			}
			conn.Close()
			return
		}
	}

	// Hostname verification: verify the claimed hostname matches the remote IP.
	if s.resolveHostname != nil {
		remoteHost, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
		if expected := s.resolveHostname(remoteHost); expected != "" && expected != hs.Hostname {
			log.Printf("hostname mismatch: peer at %s claims to be %q but Tailscale says %q", remoteHost, hs.Hostname, expected)
			conn.Close()
			return
		}
	}

	shared, err := crypto.SharedSecret(s.keyPair.PrivateKey, peerPub)
	if err != nil {
		conn.Close()
		return
	}

	c := &Connection{
		Conn:          conn,
		PeerHostname:  hs.Hostname,
		PeerPublicKey: peerPub,
		SharedSecret:  shared,
	}

	s.mu.Lock()
	if old, ok := s.conns[hs.Hostname]; ok && old != c {
		old.Conn.Close()
	}
	s.conns[hs.Hostname] = c
	s.mu.Unlock()

	if s.onConnect != nil {
		s.onConnect(c)
	}

	// Read loop
	s.readLoop(c)
}

func (s *Server) readLoop(c *Connection) {
	defer func() {
		c.Conn.Close()
		s.mu.Lock()
		removed := false
		if current, ok := s.conns[c.PeerHostname]; ok && current == c {
			delete(s.conns, c.PeerHostname)
			removed = true
		}
		s.mu.Unlock()
		if removed && s.onDisconnect != nil {
			s.onDisconnect(c.PeerHostname)
		}
	}()

	for {
		c.Conn.SetReadDeadline(time.Now().Add(ReadTimeout))
		env, err := protocol.ReadMessage(c.Conn)
		if err != nil {
			return
		}

		if s.onMessage != nil {
			s.onMessage(c, env)
		}
	}
}

// GetConnection returns an existing connection to a peer.
func (s *Server) GetConnection(hostname string) *Connection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.conns[hostname]
}

// AddConnection registers an outbound connection.
func (s *Server) AddConnection(c *Connection) {
	s.mu.Lock()
	if old, ok := s.conns[c.PeerHostname]; ok && old != c {
		old.Conn.Close()
	}
	s.conns[c.PeerHostname] = c
	s.mu.Unlock()

	// Start reading
	go s.readLoop(c)
}

// Connections returns all active connections.
func (s *Server) Connections() map[string]*Connection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]*Connection, len(s.conns))
	for k, v := range s.conns {
		result[k] = v
	}
	return result
}

// Stop shuts down the server.
func (s *Server) Stop() {
	s.stopOnce.Do(func() { close(s.stopCh) })
	s.listener.Close()
	s.mu.Lock()
	for _, c := range s.conns {
		c.Conn.Close()
	}
	s.mu.Unlock()
}

package net

import (
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/NathanBhanji/tail-chat/internal/crypto"
	"github.com/NathanBhanji/tail-chat/internal/protocol"
)

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
	listener   net.Listener
	keyPair    *crypto.KeyPair
	hostname   string
	mu         sync.RWMutex
	conns      map[string]*Connection // hostname -> connection
	onConnect  func(*Connection)
	onMessage  func(*Connection, *protocol.Envelope)
	stopCh     chan struct{}
}

// NewServer creates a TCP server bound to the given address.
func NewServer(addr string, kp *crypto.KeyPair, hostname string) (*Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}

	return &Server{
		listener: ln,
		keyPair:  kp,
		hostname: hostname,
		conns:    make(map[string]*Connection),
		stopCh:   make(chan struct{}),
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

// Addr returns the server's listen address.
func (s *Server) Addr() string {
	return s.listener.Addr().String()
}

// Start begins accepting connections.
func (s *Server) Start() {
	go s.acceptLoop()
}

func (s *Server) acceptLoop() {
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
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
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

	// Derive shared secret
	var peerPub [32]byte
	copy(peerPub[:], hs.PublicKey)

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
		// Only remove if this connection is still the active one for this peer.
		// A newer connection may have replaced us in the map.
		if current, ok := s.conns[c.PeerHostname]; ok && current == c {
			delete(s.conns, c.PeerHostname)
		}
		s.mu.Unlock()
	}()

	for {
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
	close(s.stopCh)
	s.listener.Close()
	s.mu.Lock()
	for _, c := range s.conns {
		c.Conn.Close()
	}
	s.mu.Unlock()
}

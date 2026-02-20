package discovery

import (
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

// Peer represents a machine on the tailnet.
type Peer struct {
	Hostname        string
	DNSName         string
	TailscaleIP     string
	Online          bool
	OS              string
	IsSelf          bool
	RunningTailchat bool // true if tailchat is listening on port 9377
}

// tailscaleStatus maps the JSON output of `tailscale status --json`.
type tailscaleStatus struct {
	Self *tailscalePeer            `json:"Self"`
	Peer map[string]*tailscalePeer `json:"Peer"`
}

type tailscalePeer struct {
	HostName     string   `json:"HostName"`
	DNSName      string   `json:"DNSName"`
	TailscaleIPs []string `json:"TailscaleIPs"`
	Online       bool     `json:"Online"`
	OS           string   `json:"OS"`
	UserID       int64    `json:"UserID"`
}

// tailscaleBin returns the path to the tailscale CLI binary.
// On macOS, GUI apps don't inherit the shell PATH, so we check
// common locations explicitly.
func tailscaleBin() string {
	// Try PATH first (works from terminal)
	if p, err := exec.LookPath("tailscale"); err == nil {
		return p
	}
	// macOS app bundle location
	candidates := []string{
		"/Applications/Tailscale.app/Contents/MacOS/Tailscale",
		"/usr/local/bin/tailscale",
		"/opt/homebrew/bin/tailscale",
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
	}
	return "tailscale" // fallback
}

// GetSelfIP returns this machine's Tailscale IP.
func GetSelfIP() (string, string, error) {
	out, err := exec.Command(tailscaleBin(), "status", "--json").Output()
	if err != nil {
		return "", "", fmt.Errorf("tailscale status: %w", err)
	}

	var status tailscaleStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return "", "", fmt.Errorf("parse status: %w", err)
	}

	if status.Self == nil || len(status.Self.TailscaleIPs) == 0 {
		return "", "", fmt.Errorf("no self IP found")
	}

	return status.Self.TailscaleIPs[0], status.Self.HostName, nil
}

// GetPeers returns all peers on the tailnet.
func GetPeers() ([]Peer, error) {
	out, err := exec.Command(tailscaleBin(), "status", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("tailscale status: %w", err)
	}

	var status tailscaleStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return nil, fmt.Errorf("parse status: %w", err)
	}

	var peers []Peer

	for _, p := range status.Peer {
		if len(p.TailscaleIPs) == 0 {
			continue
		}
		peers = append(peers, Peer{
			Hostname:    p.HostName,
			DNSName:     strings.TrimSuffix(p.DNSName, "."),
			TailscaleIP: p.TailscaleIPs[0],
			Online:      p.Online,
			OS:          p.OS,
		})
	}

	// Probe online peers for tailchat on port 9377
	probePeers(peers)

	// Sort: tailchat running first, then online, then alphabetical
	sort.Slice(peers, func(i, j int) bool {
		if peers[i].RunningTailchat != peers[j].RunningTailchat {
			return peers[i].RunningTailchat
		}
		if peers[i].Online != peers[j].Online {
			return peers[i].Online
		}
		return peers[i].Hostname < peers[j].Hostname
	})

	return peers, nil
}

// probePeers checks which online peers have tailchat listening on port 9377.
func probePeers(peers []Peer) {
	var wg sync.WaitGroup
	for i := range peers {
		if !peers[i].Online {
			continue
		}
		wg.Add(1)
		go func(p *Peer) {
			defer wg.Done()
			addr := net.JoinHostPort(p.TailscaleIP, "9377")
			conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
			if err == nil {
				conn.Close()
				p.RunningTailchat = true
			}
		}(&peers[i])
	}
	wg.Wait()
}

// Watcher periodically refreshes the peer list.
type Watcher struct {
	mu       sync.RWMutex
	peers    []Peer
	selfIP   string
	selfHost string
	interval time.Duration
	stopCh   chan struct{}
	onChange func([]Peer)
}

// NewWatcher creates a peer watcher that refreshes at the given interval.
func NewWatcher(interval time.Duration, onChange func([]Peer)) *Watcher {
	return &Watcher{
		interval: interval,
		stopCh:   make(chan struct{}),
		onChange: onChange,
	}
}

// Start begins periodic peer discovery.
func (w *Watcher) Start() error {
	// Initial fetch
	ip, host, err := GetSelfIP()
	if err != nil {
		return err
	}
	w.selfIP = ip
	w.selfHost = host

	peers, err := GetPeers()
	if err != nil {
		return err
	}
	w.mu.Lock()
	w.peers = peers
	w.mu.Unlock()

	if w.onChange != nil {
		w.onChange(peers)
	}

	go w.loop()
	return nil
}

func (w *Watcher) loop() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			peers, err := GetPeers()
			if err != nil {
				continue
			}
			w.mu.Lock()
			w.peers = peers
			w.mu.Unlock()
			if w.onChange != nil {
				w.onChange(peers)
			}
		case <-w.stopCh:
			return
		}
	}
}

// Stop stops the watcher.
func (w *Watcher) Stop() {
	close(w.stopCh)
}

// Peers returns the current peer list.
func (w *Watcher) Peers() []Peer {
	w.mu.RLock()
	defer w.mu.RUnlock()
	result := make([]Peer, len(w.peers))
	copy(result, w.peers)
	return result
}

// SelfIP returns this node's Tailscale IP.
func (w *Watcher) SelfIP() string {
	return w.selfIP
}

// SelfHostname returns this node's hostname.
func (w *Watcher) SelfHostname() string {
	return w.selfHost
}

// NewTestWatcher creates a Watcher with fixed data for testing (no Tailscale needed).
func NewTestWatcher(selfHost string, peers []Peer) *Watcher {
	return &Watcher{
		selfHost: selfHost,
		peers:    peers,
		stopCh:   make(chan struct{}),
	}
}

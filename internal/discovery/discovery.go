package discovery

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

// Peer represents a machine on the tailnet.
type Peer struct {
	Hostname    string
	DNSName     string
	TailscaleIP string
	Online      bool
	OS          string
	IsSelf      bool
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

// GetSelfIP returns this machine's Tailscale IP.
func GetSelfIP() (string, string, error) {
	out, err := exec.Command("tailscale", "status", "--json").Output()
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
	out, err := exec.Command("tailscale", "status", "--json").Output()
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

	// Sort: online first, then alphabetical
	sort.Slice(peers, func(i, j int) bool {
		if peers[i].Online != peers[j].Online {
			return peers[i].Online
		}
		return peers[i].Hostname < peers[j].Hostname
	})

	return peers, nil
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

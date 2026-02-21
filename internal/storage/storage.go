package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// StoredMessage is the on-disk representation of a chat message.
type StoredMessage struct {
	ID        string            `json:"id"`
	Sender    string            `json:"sender"`
	Content   string            `json:"content"`
	Timestamp time.Time         `json:"timestamp"`
	IsOwn     bool              `json:"is_own"`
	GroupID   string            `json:"group_id,omitempty"`
	Delivered bool              `json:"delivered"`
	Read      bool              `json:"read"`
	Reactions []StoredReaction  `json:"reactions,omitempty"`
	FileInfo  *StoredFileInfo   `json:"file_info,omitempty"`
}

// StoredReaction is a reaction on a message.
type StoredReaction struct {
	Emoji  string `json:"emoji"`
	Sender string `json:"sender"`
}

// StoredFileInfo persists file transfer state.
type StoredFileInfo struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	State    int    `json:"state"`
	Error    string `json:"error,omitempty"`
	Path     string `json:"path,omitempty"`
}

// StoredGroup is the on-disk representation of a group.
type StoredGroup struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Members []string `json:"members"`
}

// Store handles reading and writing chat data to disk.
type Store struct {
	mu      sync.Mutex
	baseDir string
}

// New creates a store. baseDir defaults to ~/.config/tailchat if empty.
func New(baseDir string) (*Store, error) {
	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		baseDir = filepath.Join(home, ".config", "tailchat")
	}
	for _, sub := range []string{"messages", "groups"} {
		if err := os.MkdirAll(filepath.Join(baseDir, sub), 0700); err != nil {
			return nil, err
		}
	}
	return &Store{baseDir: baseDir}, nil
}

func chatKeyToFilename(chatKey string) string {
	return strings.ReplaceAll(chatKey, ":", "_") + ".json"
}

// SaveMessages persists messages for a chat key.
func (s *Store) SaveMessages(chatKey string, msgs []StoredMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.baseDir, "messages", chatKeyToFilename(chatKey))
	data, err := json.MarshalIndent(msgs, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// LoadMessages loads messages for a chat key.
func (s *Store) LoadMessages(chatKey string) ([]StoredMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.baseDir, "messages", chatKeyToFilename(chatKey))
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var msgs []StoredMessage
	if err := json.Unmarshal(data, &msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

// SaveGroups persists all groups.
func (s *Store) SaveGroups(groups []StoredGroup) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.baseDir, "groups", "groups.json")
	data, err := json.MarshalIndent(groups, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// LoadGroups loads all groups.
func (s *Store) LoadGroups() ([]StoredGroup, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.baseDir, "groups", "groups.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var groups []StoredGroup
	if err := json.Unmarshal(data, &groups); err != nil {
		return nil, err
	}
	return groups, nil
}

// Search finds messages matching a query across all chats.
func (s *Store) Search(query string) (map[string][]StoredMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	results := make(map[string][]StoredMessage)
	dir := filepath.Join(s.baseDir, "messages")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return results, nil
		}
		return nil, err
	}

	lowerQuery := strings.ToLower(query)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var msgs []StoredMessage
		if err := json.Unmarshal(data, &msgs); err != nil {
			continue
		}
		chatKey := strings.TrimSuffix(entry.Name(), ".json")
		chatKey = strings.Replace(chatKey, "group_", "group:", 1)

		for _, msg := range msgs {
			if strings.Contains(strings.ToLower(msg.Content), lowerQuery) ||
				strings.Contains(strings.ToLower(msg.Sender), lowerQuery) {
				results[chatKey] = append(results[chatKey], msg)
			}
		}
	}
	return results, nil
}

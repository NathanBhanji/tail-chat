package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// KeyPair holds an X25519 public/private key pair.
type KeyPair struct {
	PublicKey  [32]byte `json:"public_key"`
	PrivateKey [32]byte `json:"private_key"`
}

// GenerateKeyPair creates a new random X25519 key pair.
func GenerateKeyPair() (*KeyPair, error) {
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		return nil, fmt.Errorf("generate private key: %w", err)
	}

	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("derive public key: %w", err)
	}

	kp := &KeyPair{}
	copy(kp.PrivateKey[:], priv[:])
	copy(kp.PublicKey[:], pub)
	return kp, nil
}

// SharedSecret performs X25519 ECDH and derives a 32-byte key via HKDF-SHA256.
// The salt is the two public keys concatenated in sorted order for determinism.
func SharedSecret(privateKey, peerPublicKey [32]byte) ([32]byte, error) {
	raw, err := curve25519.X25519(privateKey[:], peerPublicKey[:])
	if err != nil {
		return [32]byte{}, fmt.Errorf("ecdh: %w", err)
	}

	// Derive the public key from the private key for the salt computation.
	ownPub, err := curve25519.X25519(privateKey[:], curve25519.Basepoint)
	if err != nil {
		return [32]byte{}, fmt.Errorf("derive own public key: %w", err)
	}

	// Salt = sorted(ownPub || peerPub) for deterministic derivation.
	pubs := [][]byte{ownPub, peerPublicKey[:]}
	sort.Slice(pubs, func(i, j int) bool {
		for k := 0; k < len(pubs[i]); k++ {
			if pubs[i][k] != pubs[j][k] {
				return pubs[i][k] < pubs[j][k]
			}
		}
		return false
	})
	salt := append(pubs[0], pubs[1]...)

	hkdfReader := hkdf.New(sha256.New, raw, salt, []byte("tailchat-v1"))
	var result [32]byte
	if _, err := io.ReadFull(hkdfReader, result[:]); err != nil {
		return [32]byte{}, fmt.Errorf("hkdf: %w", err)
	}
	return result, nil
}

// Encrypt encrypts plaintext using XChaCha20-Poly1305.
// Returns nonce || ciphertext.
func Encrypt(key [32]byte, plaintext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key[:])
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	return aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts ciphertext produced by Encrypt.
// Expects nonce || ciphertext.
func Decrypt(key [32]byte, ciphertext []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key[:])
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	nonceSize := aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}

// keyFile is the JSON structure stored on disk.
type keyFile struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
}

// configDir returns ~/.tailchat and ensures it exists.
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".tailchat")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

// SaveKeyPair persists a key pair to ~/.tailchat/identity.json using atomic write.
func SaveKeyPair(kp *KeyPair) error {
	dir, err := configDir()
	if err != nil {
		return err
	}

	kf := keyFile{
		PublicKey:  hex.EncodeToString(kp.PublicKey[:]),
		PrivateKey: hex.EncodeToString(kp.PrivateKey[:]),
	}

	data, err := json.MarshalIndent(kf, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write: write to temp file then rename.
	target := filepath.Join(dir, "identity.json")
	tmp, err := os.CreateTemp(dir, ".identity-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, 0600); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// LoadKeyPair reads the key pair from ~/.tailchat/identity.json.
func LoadKeyPair() (*KeyPair, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filepath.Join(dir, "identity.json"))
	if err != nil {
		return nil, err
	}

	var kf keyFile
	if err := json.Unmarshal(data, &kf); err != nil {
		return nil, err
	}

	pubBytes, err := hex.DecodeString(kf.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("decode public key: %w", err)
	}
	if len(pubBytes) != 32 {
		return nil, fmt.Errorf("invalid public key length: %d (expected 32)", len(pubBytes))
	}
	privBytes, err := hex.DecodeString(kf.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}
	if len(privBytes) != 32 {
		return nil, fmt.Errorf("invalid private key length: %d (expected 32)", len(privBytes))
	}

	kp := &KeyPair{}
	copy(kp.PublicKey[:], pubBytes)
	copy(kp.PrivateKey[:], privBytes)
	return kp, nil
}

// LoadOrGenerateKeyPair loads an existing identity or creates a new one.
// It only generates a new identity when the file does not exist. If the file
// exists but is corrupt, an error is returned instead of silently regenerating.
func LoadOrGenerateKeyPair() (*KeyPair, error) {
	kp, err := LoadKeyPair()
	if err == nil {
		return kp, nil
	}

	// Only generate a new identity if the file doesn't exist.
	if !errors.Is(err, os.ErrNotExist) {
		// Check if the identity file actually exists on disk.
		dir, dirErr := configDir()
		if dirErr == nil {
			if _, statErr := os.Stat(filepath.Join(dir, "identity.json")); statErr == nil {
				return nil, fmt.Errorf("identity.json is corrupt: %w (delete it manually to regenerate)", err)
			}
		}
	}

	kp, err = GenerateKeyPair()
	if err != nil {
		return nil, err
	}

	if err := SaveKeyPair(kp); err != nil {
		return nil, fmt.Errorf("save new keypair: %w", err)
	}

	return kp, nil
}

// PublicKeyHex returns the public key as a hex string (for display/sharing).
func (kp *KeyPair) PublicKeyHex() string {
	return hex.EncodeToString(kp.PublicKey[:])
}

// --- TOFU Key Pinning ---

// ErrKeyChanged is returned when a known peer presents a different public key.
var ErrKeyChanged = errors.New("peer public key has changed (possible MITM)")

// KnownKeys manages TOFU key pinning for peers.
type KnownKeys struct {
	mu   sync.Mutex
	keys map[string]string // hostname -> hex-encoded public key
	path string
}

// knownKeysFile is the on-disk format.
type knownKeysFile struct {
	Keys map[string]string `json:"keys"`
}

// LoadKnownKeys loads or creates the known keys store.
func LoadKnownKeys() (*KnownKeys, error) {
	dir, err := configDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "known_keys.json")
	kk := &KnownKeys{
		keys: make(map[string]string),
		path: path,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return kk, nil
		}
		return nil, fmt.Errorf("read known keys: %w", err)
	}

	var f knownKeysFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse known keys: %w", err)
	}
	if f.Keys != nil {
		kk.keys = f.Keys
	}
	return kk, nil
}

// Verify checks a peer's public key against the known store.
// First contact: stores the key and returns nil.
// Known peer, same key: returns nil.
// Known peer, different key: returns ErrKeyChanged.
func (kk *KnownKeys) Verify(hostname string, pubKey [32]byte) error {
	hexKey := hex.EncodeToString(pubKey[:])

	kk.mu.Lock()
	defer kk.mu.Unlock()

	if stored, ok := kk.keys[hostname]; ok {
		if stored != hexKey {
			return fmt.Errorf("%w: %s (expected %s..., got %s...)",
				ErrKeyChanged, hostname, stored[:8], hexKey[:8])
		}
		return nil
	}

	// First contact — trust on first use
	kk.keys[hostname] = hexKey
	return kk.save()
}

func (kk *KnownKeys) save() error {
	f := knownKeysFile{Keys: kk.keys}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write
	tmp, err := os.CreateTemp(filepath.Dir(kk.path), ".known_keys-*.json.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	tmp.Close()
	os.Chmod(tmpName, 0600)
	return os.Rename(tmpName, kk.path)
}

package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
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

// SharedSecret performs X25519 ECDH to derive a shared secret.
func SharedSecret(privateKey, peerPublicKey [32]byte) ([32]byte, error) {
	shared, err := curve25519.X25519(privateKey[:], peerPublicKey[:])
	if err != nil {
		return [32]byte{}, fmt.Errorf("ecdh: %w", err)
	}
	var result [32]byte
	copy(result[:], shared)
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

// SaveKeyPair persists a key pair to ~/.tailchat/identity.json.
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

	return os.WriteFile(filepath.Join(dir, "identity.json"), data, 0600)
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
	privBytes, err := hex.DecodeString(kf.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}

	kp := &KeyPair{}
	copy(kp.PublicKey[:], pubBytes)
	copy(kp.PrivateKey[:], privBytes)
	return kp, nil
}

// LoadOrGenerateKeyPair loads an existing identity or creates a new one.
func LoadOrGenerateKeyPair() (*KeyPair, error) {
	kp, err := LoadKeyPair()
	if err == nil {
		return kp, nil
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

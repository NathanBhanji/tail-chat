package crypto

import (
	"bytes"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	if kp.PublicKey == [32]byte{} {
		t.Fatal("public key is zero")
	}
	if kp.PrivateKey == [32]byte{} {
		t.Fatal("private key is zero")
	}

	// Two keypairs should be different
	kp2, _ := GenerateKeyPair()
	if kp.PublicKey == kp2.PublicKey {
		t.Fatal("two keypairs have same public key")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	key := [32]byte{}
	copy(key[:], []byte("test-key-for-encryption-1234567"))

	plaintext := []byte("hello tailchat, this is a secret message!")

	ciphertext, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("ciphertext equals plaintext")
	}

	decrypted, err := Decrypt(key, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted != plaintext: got %q", decrypted)
	}
}

func TestSharedSecret(t *testing.T) {
	alice, _ := GenerateKeyPair()
	bob, _ := GenerateKeyPair()

	// Alice computes shared secret with Bob's public key
	secretA, err := SharedSecret(alice.PrivateKey, bob.PublicKey)
	if err != nil {
		t.Fatalf("SharedSecret(alice, bob): %v", err)
	}

	// Bob computes shared secret with Alice's public key
	secretB, err := SharedSecret(bob.PrivateKey, alice.PublicKey)
	if err != nil {
		t.Fatalf("SharedSecret(bob, alice): %v", err)
	}

	// Both should derive the same secret
	if secretA != secretB {
		t.Fatal("shared secrets don't match")
	}
}

func TestE2EEncryption(t *testing.T) {
	// Full flow: keygen -> ECDH -> encrypt -> decrypt
	alice, _ := GenerateKeyPair()
	bob, _ := GenerateKeyPair()

	shared, _ := SharedSecret(alice.PrivateKey, bob.PublicKey)

	msg := []byte("hey bob, this is alice speaking over e2e encryption!")
	ciphertext, err := Encrypt(shared, msg)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Bob decrypts using same shared secret
	sharedBob, _ := SharedSecret(bob.PrivateKey, alice.PublicKey)
	decrypted, err := Decrypt(sharedBob, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if !bytes.Equal(decrypted, msg) {
		t.Fatalf("got %q, want %q", decrypted, msg)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1 := [32]byte{}
	key2 := [32]byte{}
	copy(key1[:], []byte("key-one-1234567890123456"))
	copy(key2[:], []byte("key-two-1234567890123456"))

	ciphertext, _ := Encrypt(key1, []byte("secret"))

	_, err := Decrypt(key2, ciphertext)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}
}

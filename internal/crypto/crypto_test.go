package crypto

import (
	"bytes"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	priv, pub, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}
	if len(priv) != 32 {
		t.Fatalf("expected private key length 32, got %d", len(priv))
	}
	if len(pub) != 32 {
		t.Fatalf("expected public key length 32, got %d", len(pub))
	}
}

func TestSharedSecretSymmetry(t *testing.T) {
	privA, pubA, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair A failed: %v", err)
	}
	privB, pubB, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair B failed: %v", err)
	}

	secretA, err := ComputeSharedSecret(privA, pubB)
	if err != nil {
		t.Fatalf("ComputeSharedSecret A failed: %v", err)
	}
	secretB, err := ComputeSharedSecret(privB, pubA)
	if err != nil {
		t.Fatalf("ComputeSharedSecret B failed: %v", err)
	}

	if !bytes.Equal(secretA, secretB) {
		t.Fatal("shared secrets do not match")
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	privA, pubA, _ := GenerateKeyPair()
	privB, pubB, _ := GenerateKeyPair()

	key, _ := ComputeSharedSecret(privA, pubB)

	plaintext := []byte("hello this is a secret message")
	ciphertext, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Ciphertext should differ from plaintext
	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("ciphertext equals plaintext")
	}

	// Decrypt with same shared secret (from other side)
	key2, _ := ComputeSharedSecret(privB, pubA)
	decrypted, err := Decrypt(key2, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted text does not match: got %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	privA, _, _ := GenerateKeyPair()
	_, pubB, _ := GenerateKeyPair()
	_, pubC, _ := GenerateKeyPair()

	key, _ := ComputeSharedSecret(privA, pubB)
	wrongKey, _ := ComputeSharedSecret(privA, pubC)

	plaintext := []byte("secret")
	ciphertext, _ := Encrypt(key, plaintext)

	_, err := Decrypt(wrongKey, ciphertext)
	if err == nil {
		t.Fatal("expected decryption to fail with wrong key")
	}
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	privA, _, _ := GenerateKeyPair()
	_, pubB, _ := GenerateKeyPair()
	key, _ := ComputeSharedSecret(privA, pubB)

	plaintext := []byte("same message")
	ct1, _ := Encrypt(key, plaintext)
	ct2, _ := Encrypt(key, plaintext)

	if bytes.Equal(ct1, ct2) {
		t.Fatal("encrypting the same plaintext produced identical ciphertexts (nonce reuse)")
	}
}

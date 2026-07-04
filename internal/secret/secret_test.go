package secret

import (
	"context"
	"errors"
	"testing"
)

func TestLocalManagerEncryptDecrypt(t *testing.T) {
	m := testManager(t)
	ctx := context.Background()

	ciphertext, err := m.Encrypt(ctx, "admin_model_config:chat", "sk-test-secret")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	if ciphertext.Value == "" || ciphertext.Nonce == "" || ciphertext.Algorithm != AlgorithmAES256GCM {
		t.Fatalf("ciphertext missing fields: %+v", ciphertext)
	}

	plain, err := m.Decrypt(ctx, "admin_model_config:chat", ciphertext)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if plain != "sk-test-secret" {
		t.Fatalf("Decrypt() = %q", plain)
	}
}

func TestLocalManagerPurposeIsolation(t *testing.T) {
	m := testManager(t)
	ctx := context.Background()

	ciphertext, err := m.Encrypt(ctx, "admin_model_config:chat", "sk-test-secret")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	_, err = m.Decrypt(ctx, "admin_model_config:rerank", ciphertext)
	if !errors.Is(err, ErrInvalidCiphertext) {
		t.Fatalf("Decrypt() error = %v, want ErrInvalidCiphertext", err)
	}
}

func TestLocalManagerMaskAndFingerprint(t *testing.T) {
	m := testManager(t)

	if got := m.Mask("sk-1234567890"); got != "sk-1****7890" {
		t.Fatalf("Mask() = %q", got)
	}
	if got := m.Mask("short"); got != "****" {
		t.Fatalf("Mask(short) = %q", got)
	}
	if got := m.Fingerprint("sk-1234567890"); got == "" || len(got) != 16 {
		t.Fatalf("Fingerprint() = %q", got)
	}
	if m.Fingerprint("sk-1234567890") != m.Fingerprint("sk-1234567890") {
		t.Fatal("Fingerprint() should be stable")
	}
}

func testManager(t *testing.T) *LocalManager {
	t.Helper()
	key := []byte("0123456789abcdef0123456789abcdef")
	m, err := NewLocalManager(key, "test-key", "v1")
	if err != nil {
		t.Fatalf("NewLocalManager() error = %v", err)
	}
	return m
}

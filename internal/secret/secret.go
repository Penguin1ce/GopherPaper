// Package secret centralizes reversible secret protection for values that must
// be used by the backend but must not be stored as plaintext in the database.
package secret

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	AlgorithmAES256GCM = "AES-256-GCM"

	defaultKeyPath = "data/secrets/master.key"
	envKey         = "GOPHERPAPER_SECRET_KEY"
	envKeyFile     = "GOPHERPAPER_SECRET_KEY_FILE"
)

var (
	defaultMu      sync.Mutex
	defaultManager Manager

	ErrInvalidCiphertext = errors.New("secret: invalid ciphertext")
)

type Ciphertext struct {
	Value      string
	Nonce      string
	KeyID      string
	KeyVersion string
	Algorithm  string
}

type Manager interface {
	Encrypt(ctx context.Context, purpose string, plaintext string) (Ciphertext, error)
	Decrypt(ctx context.Context, purpose string, ciphertext Ciphertext) (string, error)
	Fingerprint(plaintext string) string
	Mask(plaintext string) string
}

func Default() (Manager, error) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultManager != nil {
		return defaultManager, nil
	}
	m, err := NewLocalManagerFromEnv()
	if err != nil {
		return nil, err
	}
	defaultManager = m
	return m, nil
}

func SetDefaultForTest(m Manager) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultManager = m
}

func NewLocalManagerFromEnv() (*LocalManager, error) {
	if raw := strings.TrimSpace(os.Getenv(envKey)); raw != "" {
		key, err := parseKey(raw)
		if err != nil {
			return nil, fmt.Errorf("secret: invalid %s: %w", envKey, err)
		}
		return NewLocalManager(key, "env:"+envKey, "v1")
	}

	path := strings.TrimSpace(os.Getenv(envKeyFile))
	if path == "" {
		path = defaultKeyPath
	}
	key, err := loadOrCreateKeyFile(path)
	if err != nil {
		return nil, err
	}
	return NewLocalManager(key, "file:"+filepath.ToSlash(path), "v1")
}

type LocalManager struct {
	aead       cipher.AEAD
	key        []byte
	keyID      string
	keyVersion string
}

func NewLocalManager(key []byte, keyID, keyVersion string) (*LocalManager, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("AES-256-GCM key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(keyID) == "" {
		keyID = "local:" + keyFingerprint(key)
	}
	if strings.TrimSpace(keyVersion) == "" {
		keyVersion = "v1"
	}
	return &LocalManager{
		aead:       aead,
		key:        append([]byte(nil), key...),
		keyID:      keyID,
		keyVersion: keyVersion,
	}, nil
}

func (m *LocalManager) Encrypt(ctx context.Context, purpose string, plaintext string) (Ciphertext, error) {
	if err := ctx.Err(); err != nil {
		return Ciphertext{}, err
	}
	nonce := make([]byte, m.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return Ciphertext{}, fmt.Errorf("secret: generate nonce failed: %w", err)
	}
	sealed := m.aead.Seal(nil, nonce, []byte(plaintext), aad(purpose))
	return Ciphertext{
		Value:      base64.StdEncoding.EncodeToString(sealed),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		KeyID:      m.keyID,
		KeyVersion: m.keyVersion,
		Algorithm:  AlgorithmAES256GCM,
	}, nil
}

func (m *LocalManager) Decrypt(ctx context.Context, purpose string, ciphertext Ciphertext) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if ciphertext.Algorithm != "" && ciphertext.Algorithm != AlgorithmAES256GCM {
		return "", fmt.Errorf("%w: unsupported algorithm %s", ErrInvalidCiphertext, ciphertext.Algorithm)
	}
	nonce, err := base64.StdEncoding.DecodeString(ciphertext.Nonce)
	if err != nil {
		return "", fmt.Errorf("%w: invalid nonce", ErrInvalidCiphertext)
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext.Value)
	if err != nil {
		return "", fmt.Errorf("%w: invalid value", ErrInvalidCiphertext)
	}
	opened, err := m.aead.Open(nil, nonce, raw, aad(purpose))
	if err != nil {
		return "", fmt.Errorf("%w: authentication failed", ErrInvalidCiphertext)
	}
	return string(opened), nil
}

func (m *LocalManager) Fingerprint(plaintext string) string {
	if strings.TrimSpace(plaintext) == "" {
		return ""
	}
	mac := hmac.New(sha256.New, m.key)
	_, _ = mac.Write([]byte(plaintext))
	return hex.EncodeToString(mac.Sum(nil))[:16]
}

func (m *LocalManager) Mask(plaintext string) string {
	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" {
		return ""
	}
	if len(plaintext) <= 8 {
		return "****"
	}
	return plaintext[:4] + "****" + plaintext[len(plaintext)-4:]
}

func parseKey(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "base64:") {
		return parseBase64Key(strings.TrimPrefix(raw, "base64:"))
	}
	if strings.HasPrefix(raw, "hex:") {
		key, err := hex.DecodeString(strings.TrimPrefix(raw, "hex:"))
		if err != nil {
			return nil, err
		}
		return validateKey(key)
	}
	if key, err := base64.StdEncoding.DecodeString(raw); err == nil && len(key) == 32 {
		return key, nil
	}
	if key, err := hex.DecodeString(raw); err == nil && len(key) == 32 {
		return key, nil
	}
	return validateKey([]byte(raw))
}

func parseBase64Key(raw string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return nil, err
	}
	return validateKey(key)
}

func validateKey(key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be exactly 32 bytes")
	}
	return append([]byte(nil), key...), nil
}

func loadOrCreateKeyFile(path string) ([]byte, error) {
	if raw, err := os.ReadFile(path); err == nil {
		return parseBase64Key(string(raw))
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("secret: read key file %s failed: %w", path, err)
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("secret: generate key failed: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("secret: create key directory failed: %w", err)
	}
	content := []byte(base64.StdEncoding.EncodeToString(key) + "\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return nil, fmt.Errorf("secret: write key file %s failed: %w", path, err)
	}
	return key, nil
}

func keyFingerprint(key []byte) string {
	sum := sha256.Sum256(key)
	return hex.EncodeToString(sum[:])[:16]
}

func aad(purpose string) []byte {
	return []byte("gopherpaper:" + strings.TrimSpace(purpose))
}

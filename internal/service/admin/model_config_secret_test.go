package admin

import (
	"context"
	"testing"

	"GopherPaper/internal/model"
	"GopherPaper/internal/secret"
)

func TestProtectModelConfigAPIKeyEncryptsAndClearsPlaintext(t *testing.T) {
	m := fixedSecretManager(t)
	secret.SetDefaultForTest(m)
	t.Cleanup(func() { secret.SetDefaultForTest(nil) })

	row := model.SystemModelConfig{
		Role:   "chat",
		APIKey: "sk-test-secret",
	}
	if err := protectModelConfigAPIKey(context.Background(), &row); err != nil {
		t.Fatalf("protectModelConfigAPIKey() error = %v", err)
	}

	if row.APIKey != "" {
		t.Fatalf("APIKey plaintext was not cleared: %q", row.APIKey)
	}
	if row.APIKeyCiphertext == "" || row.APIKeyNonce == "" || row.APIKeyAlgorithm != secret.AlgorithmAES256GCM {
		t.Fatalf("secret fields missing: %+v", row)
	}
	if row.APIKeyMask != "sk-t****cret" {
		t.Fatalf("APIKeyMask = %q", row.APIKeyMask)
	}
	if row.APIKeyFingerprint == "" {
		t.Fatal("APIKeyFingerprint is empty")
	}

	plain, err := m.Decrypt(context.Background(), modelConfigSecretPurpose("chat"), modelConfigCiphertext(row))
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}
	if plain != "sk-test-secret" {
		t.Fatalf("Decrypt() = %q", plain)
	}
}

func TestProtectModelConfigAPIKeyClearsSecretFields(t *testing.T) {
	row := model.SystemModelConfig{
		Role:              "chat",
		APIKey:            "",
		APIKeyCiphertext:  "cipher",
		APIKeyNonce:       "nonce",
		APIKeyKeyID:       "key",
		APIKeyKeyVersion:  "v1",
		APIKeyAlgorithm:   secret.AlgorithmAES256GCM,
		APIKeyMask:        "sk****test",
		APIKeyFingerprint: "fingerprint",
	}
	if err := protectModelConfigAPIKey(context.Background(), &row); err != nil {
		t.Fatalf("protectModelConfigAPIKey() error = %v", err)
	}
	if modelConfigHasAPIKey(row) {
		t.Fatalf("modelConfigHasAPIKey() = true after clear: %+v", row)
	}
	if row.APIKeyCiphertext != "" || row.APIKeyMask != "" || row.APIKeyFingerprint != "" {
		t.Fatalf("secret fields were not cleared: %+v", row)
	}
}

func fixedSecretManager(t *testing.T) *secret.LocalManager {
	t.Helper()
	m, err := secret.NewLocalManager([]byte("0123456789abcdef0123456789abcdef"), "test-key", "v1")
	if err != nil {
		t.Fatalf("NewLocalManager() error = %v", err)
	}
	return m
}

package platform

import (
	"os"
	"path/filepath"
	"testing"

	cryptopkg "github.com/elpic/blueprint/internal/crypto"
)

func TestRealCryptoProvider_IsEncrypted(t *testing.T) {
	c := &realCryptoProvider{}

	t.Run("encrypted file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "secret.enc")
		encrypted, err := cryptopkg.EncryptFile([]byte("top secret data"), "hunter2")
		if err != nil {
			t.Fatalf("EncryptFile: %v", err)
		}
		if err := os.WriteFile(path, encrypted, 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if !c.IsEncrypted(path) {
			t.Errorf("IsEncrypted(%q) = false, want true", path)
		}
	})

	t.Run("small plaintext file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "plain.txt")
		if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		if c.IsEncrypted(path) {
			t.Errorf("IsEncrypted(%q) = true, want false", path)
		}
	})

	t.Run("nonexistent path", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing.enc")
		if c.IsEncrypted(path) {
			t.Errorf("IsEncrypted(%q) = true, want false", path)
		}
	})
}

func TestRealCryptoProvider_EncryptDecryptRoundTrip(t *testing.T) {
	c := &realCryptoProvider{}

	dir := t.TempDir()
	input := filepath.Join(dir, "secret.txt")
	encPath := filepath.Join(dir, "nested", "secret.enc")
	output := filepath.Join(dir, "nested", "restored.txt")

	if err := os.WriteFile(input, []byte("round-trip payload"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := c.Encrypt(input, encPath, "pw"); err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if err := c.Decrypt(encPath, output, "pw"); err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	restored, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(restored) != "round-trip payload" {
		t.Errorf("round-trip mismatch: got %q, want %q", restored, "round-trip payload")
	}
}

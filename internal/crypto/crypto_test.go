package crypto

import (
	"bytes"
	"testing"
)

// ---- round-trip -----------------------------------------------------------

func TestEncryptDecryptRoundTrip(t *testing.T) {
	plaintext := []byte("secret configuration data")
	password := "correct-horse-battery-staple"

	ciphertext, err := EncryptFile(plaintext, password)
	if err != nil {
		t.Fatalf("EncryptFile() error: %v", err)
	}

	got, err := DecryptFile(ciphertext, password)
	if err != nil {
		t.Fatalf("DecryptFile() error: %v", err)
	}

	if !bytes.Equal(got, plaintext) {
		t.Errorf("round-trip mismatch\n  got:  %q\n  want: %q", got, plaintext)
	}
}

// ---- Bug: deriveKey produces the same key for the same password -----------
//
// Raw SHA-256 with no salt means every file encrypted with the same password
// uses the same AES key. An attacker who cracks one file's key cracks them all.
//
// The fix: deriveKey must accept a salt and use PBKDF2 (or Argon2) so that
// two calls with the same password but different salts produce different keys.
//
// This test fails on the original code: deriveKey ignores the salt argument,
// so both calls return the same key.

func TestDeriveKeyDiffersWithDifferentSalts(t *testing.T) {
	salt1 := make([]byte, 32)
	salt2 := make([]byte, 32)
	salt1[0] = 0x01
	salt2[0] = 0x02

	key1 := deriveKey("same-password", salt1)
	key2 := deriveKey("same-password", salt2)

	if bytes.Equal(key1, key2) {
		t.Error("BUG: deriveKey returns the same key for different salts — no salt is being used in key derivation")
	}
}

// ---- wrong password -------------------------------------------------------

func TestDecryptWrongPasswordFails(t *testing.T) {
	ciphertext, err := EncryptFile([]byte("sensitive data"), "correct-password")
	if err != nil {
		t.Fatalf("EncryptFile() error: %v", err)
	}

	_, err = DecryptFile(ciphertext, "wrong-password")
	if err == nil {
		t.Fatal("DecryptFile() should fail with wrong password, got nil error")
	}
}

// ---- truncated ciphertext -------------------------------------------------

func TestDecryptTruncatedCiphertextFails(t *testing.T) {
	ciphertext, err := EncryptFile([]byte("data"), "password")
	if err != nil {
		t.Fatalf("EncryptFile() error: %v", err)
	}

	_, err = DecryptFile(ciphertext[:10], "password")
	if err == nil {
		t.Fatal("DecryptFile() should fail on truncated ciphertext, got nil error")
	}
}

// ---- empty plaintext ------------------------------------------------------

func TestEncryptDecryptEmptyPlaintext(t *testing.T) {
	ciphertext, err := EncryptFile([]byte{}, "password")
	if err != nil {
		t.Fatalf("EncryptFile() error: %v", err)
	}

	got, err := DecryptFile(ciphertext, "password")
	if err != nil {
		t.Fatalf("DecryptFile() error: %v", err)
	}

	if len(got) != 0 {
		t.Errorf("expected empty plaintext, got %q", got)
	}
}

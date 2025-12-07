package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
)

// EncryptFile encrypts a file using AES-256-GCM and returns the encrypted data
func EncryptFile(plaintext []byte, password string) ([]byte, error) {
	// Derive a 32-byte key from the password using a simple method
	// In production, use PBKDF2, Argon2, or similar
	key := deriveKey(password)

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the plaintext
	ciphertext := aead.Seal(nonce, nonce, plaintext, nil)

	return ciphertext, nil
}

// DecryptFile decrypts a file encrypted with EncryptFile using AES-256-GCM
func DecryptFile(ciphertext []byte, password string) ([]byte, error) {
	// Derive key from password (must match encryption)
	key := deriveKey(password)

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce from ciphertext
	nonceSize := aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := ciphertext[:nonceSize]
	encryptedData := ciphertext[nonceSize:]

	// Decrypt
	plaintext, err := aead.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	return plaintext, nil
}

// deriveKey creates a 32-byte key from a password
// Uses SHA-256 hash-based derivation
// In production, use PBKDF2 or Argon2 for better security
func deriveKey(password string) []byte {
	// Use SHA-256 to derive a key from password
	hash := sha256.Sum256([]byte(password))
	return hash[:]
}

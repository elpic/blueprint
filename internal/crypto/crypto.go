package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

const saltSize = 32

// EncryptFile encrypts a file using AES-256-GCM and returns the encrypted data
func EncryptFile(plaintext []byte, password string) ([]byte, error) {
	// Generate random salt
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	key := deriveKey(password, salt)

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

	// Output: [salt | nonce | ciphertext+tag]
	out := make([]byte, 0, saltSize+len(nonce)+len(plaintext)+aead.Overhead())
	out = append(out, salt...)
	out = aead.Seal(append(out, nonce...), nonce, plaintext, nil)

	return out, nil
}

// DecryptFile decrypts a file encrypted with EncryptFile using AES-256-GCM
func DecryptFile(ciphertext []byte, password string) ([]byte, error) {
	// Extract salt, then nonce, then encrypted data
	if len(ciphertext) < saltSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	salt := ciphertext[:saltSize]
	rest := ciphertext[saltSize:]

	key := deriveKey(password, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := aead.NonceSize()
	if len(rest) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := rest[:nonceSize]
	encryptedData := rest[nonceSize:]

	// Decrypt
	plaintext, err := aead.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	return plaintext, nil
}

const pbkdf2Iterations = 260_000 // OWASP recommended minimum for PBKDF2-SHA256

// deriveKey creates a 32-byte AES key from a password and a random salt
// using PBKDF2-SHA256 with 260,000 iterations.
func deriveKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, pbkdf2Iterations, 32, sha256.New)
}

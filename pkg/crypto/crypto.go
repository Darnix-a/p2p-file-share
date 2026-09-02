package crypto

import (
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

const (
	KeySize   = 32
	NonceSize = 12
)

// KeyPair holds an ephemeral X25519 private and public key
type KeyPair struct {
	PrivateKey [32]byte
	PublicKey  [32]byte
}

// GenerateKeyPair generates a new ephemeral X25519 keypair
func GenerateKeyPair() (*KeyPair, error) {
	var priv [32]byte
	if _, err := io.ReadFull(rand.Reader, priv[:]); err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("failed to compute public key: %w", err)
	}

	var pubArr [32]byte
	copy(pubArr[:], pub)

	return &KeyPair{
		PrivateKey: priv,
		PublicKey:  pubArr,
	}, nil
}

// DeriveSessionKey computes the shared secret using ECDH and derives a 32-byte key with HKDF
func DeriveSessionKey(ourPriv [32]byte, theirPub [32]byte, roomCode string, isInitiator bool) ([]byte, error) {
	sharedSecret, err := curve25519.X25519(ourPriv[:], theirPub[:])
	if err != nil {
		return nil, fmt.Errorf("ECDH computation failed: %w", err)
	}

	// Salt derived from room code
	salt := sha256.Sum256([]byte(roomCode))
	info := []byte("p2p-drop-v1-session-key")

	kdf := hkdf.New(sha256.New, sharedSecret, salt[:], info)
	sessionKey := make([]byte, KeySize)
	if _, err := io.ReadFull(kdf, sessionKey); err != nil {
		return nil, fmt.Errorf("failed to derive session key: %w", err)
	}

	return sessionKey, nil
}

// SessionCipher wraps ChaCha20-Poly1305 AEAD for stream chunk encryption
type SessionCipher struct {
	aead cipher.AEAD
}

// NewSessionCipher creates a new SessionCipher from a 32-byte symmetric key
func NewSessionCipher(key []byte) (*SessionCipher, error) {
	if len(key) != chacha20poly1305.KeySize {
		return nil, errors.New("invalid key size for ChaCha20-Poly1305")
	}
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AEAD cipher: %w", err)
	}
	return &SessionCipher{aead: aead}, nil
}

// EncryptChunk encrypts plaintext using a 64-bit sequence counter for the nonce
func (c *SessionCipher) EncryptChunk(seq uint64, plaintext []byte, additionalData []byte) ([]byte, error) {
	nonce := make([]byte, chacha20poly1305.NonceSize)
	binary.BigEndian.PutUint64(nonce[4:], seq)

	ciphertext := c.aead.Seal(nil, nonce, plaintext, additionalData)
	return ciphertext, nil
}

// DecryptChunk decrypts ciphertext using the corresponding 64-bit sequence counter
func (c *SessionCipher) DecryptChunk(seq uint64, ciphertext []byte, additionalData []byte) ([]byte, error) {
	nonce := make([]byte, chacha20poly1305.NonceSize)
	binary.BigEndian.PutUint64(nonce[4:], seq)

	plaintext, err := c.aead.Open(nil, nonce, ciphertext, additionalData)
	if err != nil {
		return nil, fmt.Errorf("decryption/authentication failed for chunk %d: %w", seq, err)
	}
	return plaintext, nil
}

// HashData calculates SHA256 checksum hex string
func HashData(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum)
}

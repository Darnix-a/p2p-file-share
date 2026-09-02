package crypto

import (
	"bytes"
	"testing"
)

func TestCryptoECDHAndCipher(t *testing.T) {
	// 1. Generate keypairs for Alice (sender) and Bob (receiver)
	aliceKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Alice GenerateKeyPair failed: %v", err)
	}

	bobKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Bob GenerateKeyPair failed: %v", err)
	}

	roomCode := "42-crystal-dragon"

	// 2. Both derive session key
	aliceSessionKey, err := DeriveSessionKey(aliceKey.PrivateKey, bobKey.PublicKey, roomCode, true)
	if err != nil {
		t.Fatalf("Alice DeriveSessionKey failed: %v", err)
	}

	bobSessionKey, err := DeriveSessionKey(bobKey.PrivateKey, aliceKey.PublicKey, roomCode, false)
	if err != nil {
		t.Fatalf("Bob DeriveSessionKey failed: %v", err)
	}

	if !bytes.Equal(aliceSessionKey, bobSessionKey) {
		t.Fatalf("Derived session keys do not match!\nAlice: %x\nBob:   %x", aliceSessionKey, bobSessionKey)
	}

	// 3. Encrypt and decrypt chunks
	aliceCipher, err := NewSessionCipher(aliceSessionKey)
	if err != nil {
		t.Fatalf("Alice NewSessionCipher failed: %v", err)
	}

	bobCipher, err := NewSessionCipher(bobSessionKey)
	if err != nil {
		t.Fatalf("Bob NewSessionCipher failed: %v", err)
	}

	testPayload := []byte("Hello, this is a secret P2P file payload! 🚀")
	seq := uint64(1)

	ciphertext, err := aliceCipher.EncryptChunk(seq, testPayload, nil)
	if err != nil {
		t.Fatalf("EncryptChunk failed: %v", err)
	}

	decrypted, err := bobCipher.DecryptChunk(seq, ciphertext, nil)
	if err != nil {
		t.Fatalf("DecryptChunk failed: %v", err)
	}

	if !bytes.Equal(decrypted, testPayload) {
		t.Fatalf("Decrypted payload does not match original!\nGot:  %s\nWant: %s", string(decrypted), string(testPayload))
	}

	// 4. Test tampering resistance
	corrupted := make([]byte, len(ciphertext))
	copy(corrupted, ciphertext)
	corrupted[len(corrupted)-1] ^= 0xFF

	_, err = bobCipher.DecryptChunk(seq, corrupted, nil)
	if err == nil {
		t.Fatalf("Expected decryption to fail for tampered ciphertext, but it succeeded")
	}
}

func TestGenerateCode(t *testing.T) {
	code, err := GenerateCode(3)
	if err != nil {
		t.Fatalf("GenerateCode failed: %v", err)
	}
	if len(code) < 5 {
		t.Fatalf("Generated code too short: %s", code)
	}
}

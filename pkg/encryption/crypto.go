// Package encryption provides authenticated field-level encryption and
// keyed blind indexes for protected health information (PHI).
//
// A single master key (ENCRYPTION_KEY) is expanded with HKDF-SHA256 into
// independent subkeys: one for AES-256-GCM encryption and one for the
// HMAC-SHA256 blind index used to search encrypted values by exact match.
package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
)

// KeySize is the size in bytes of AES-256 keys.
const KeySize = 32

// formatV1 identifies ciphertexts produced by Cipher: 1 version byte,
// a 12 byte random nonce, then the GCM ciphertext and tag.
const formatV1 byte = 1

// ErrDecrypt is returned when a ciphertext is malformed, was produced with a
// different key, or has been tampered with.
var ErrDecrypt = errors.New("encryption: unable to decrypt value")

// ParseKey decodes a master key. It accepts the standard base64 encoding of
// 32 random bytes (for example the output of `openssl rand -base64 32`), or
// any raw string of at least 32 bytes, which is then used as HKDF input.
func ParseKey(s string) ([]byte, error) {
	if b, err := base64.StdEncoding.DecodeString(s); err == nil && len(b) == KeySize {
		return b, nil
	}
	if len(s) >= KeySize {
		return []byte(s), nil
	}
	return nil, errors.New("key must be base64 encoded 32 bytes or a string of at least 32 bytes")
}

// Keys holds the subkeys derived from a master key.
type Keys struct {
	cipher   *Cipher
	indexKey []byte
}

// NewKeys derives encryption and blind index subkeys from master.
func NewKeys(master []byte) (*Keys, error) {
	if len(master) < KeySize {
		return nil, fmt.Errorf("encryption: master key must be at least %d bytes", KeySize)
	}
	encKey, err := hkdf.Key(sha256.New, master, nil, "healthhub/phi-encryption/v1", KeySize)
	if err != nil {
		return nil, fmt.Errorf("encryption: derive encryption key: %w", err)
	}
	indexKey, err := hkdf.Key(sha256.New, master, nil, "healthhub/blind-index/v1", KeySize)
	if err != nil {
		return nil, fmt.Errorf("encryption: derive index key: %w", err)
	}
	c, err := NewCipher(encKey)
	if err != nil {
		return nil, err
	}
	return &Keys{cipher: c, indexKey: indexKey}, nil
}

// Cipher returns the AEAD used for field encryption.
func (k *Keys) Cipher() *Cipher { return k.cipher }

// BlindIndex returns a deterministic, keyed digest of value within the given
// domain (for example "family" or "email"). Equal inputs produce equal
// digests, so encrypted columns can be searched by exact match without
// storing the plaintext. Callers are responsible for normalising value.
func (k *Keys) BlindIndex(domain, value string) string {
	mac := hmac.New(sha256.New, k.indexKey)
	mac.Write([]byte(domain))
	mac.Write([]byte{0})
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil)[:16])
}

// Cipher performs AES-256-GCM authenticated encryption.
type Cipher struct {
	aead cipher.AEAD
}

// NewCipher returns a Cipher for a 32 byte key.
func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("encryption: key must be %d bytes, got %d", KeySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// Seal encrypts plaintext. The additional data aad is authenticated but not
// stored; pass the record identifier so a ciphertext cannot be moved to a
// different row without detection.
func (c *Cipher) Seal(plaintext, aad []byte) ([]byte, error) {
	nonceSize := c.aead.NonceSize()
	out := make([]byte, 1+nonceSize, 1+nonceSize+len(plaintext)+c.aead.Overhead())
	out[0] = formatV1
	if _, err := rand.Read(out[1 : 1+nonceSize]); err != nil {
		return nil, fmt.Errorf("encryption: generate nonce: %w", err)
	}
	return c.aead.Seal(out, out[1:1+nonceSize], plaintext, c.additionalData(aad)), nil
}

// Open decrypts a value produced by Seal with the same aad.
func (c *Cipher) Open(data, aad []byte) ([]byte, error) {
	nonceSize := c.aead.NonceSize()
	if len(data) < 1+nonceSize+c.aead.Overhead() || data[0] != formatV1 {
		return nil, ErrDecrypt
	}
	plaintext, err := c.aead.Open(nil, data[1:1+nonceSize], data[1+nonceSize:], c.additionalData(aad))
	if err != nil {
		return nil, ErrDecrypt
	}
	return plaintext, nil
}

// additionalData binds the format version to the caller supplied data.
func (c *Cipher) additionalData(aad []byte) []byte {
	return append([]byte{formatV1}, aad...)
}

package encryption

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func testKeys(t *testing.T) *Keys {
	t.Helper()
	k, err := NewKeys(bytes.Repeat([]byte{7}, KeySize))
	if err != nil {
		t.Fatalf("NewKeys: %v", err)
	}
	return k
}

func TestParseKey(t *testing.T) {
	raw := bytes.Repeat([]byte{0xAB}, KeySize)
	b64 := base64.StdEncoding.EncodeToString(raw)

	tests := []struct {
		name    string
		in      string
		want    []byte
		wantErr bool
	}{
		{name: "base64 32 bytes", in: b64, want: raw},
		{name: "raw 32 chars", in: strings.Repeat("k", 32), want: []byte(strings.Repeat("k", 32))},
		{name: "raw long passphrase", in: strings.Repeat("p", 50), want: []byte(strings.Repeat("p", 50))},
		{name: "too short", in: "short", wantErr: true},
		{name: "empty", in: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseKey(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("got %x, want %x", got, tt.want)
			}
		})
	}
}

func TestSealOpenRoundTrip(t *testing.T) {
	c := testKeys(t).Cipher()
	for _, msg := range [][]byte{nil, []byte(""), []byte("x"), bytes.Repeat([]byte("phi"), 10000)} {
		ct, err := c.Seal(msg, []byte("patient-1"))
		if err != nil {
			t.Fatalf("Seal: %v", err)
		}
		pt, err := c.Open(ct, []byte("patient-1"))
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if !bytes.Equal(pt, msg) {
			t.Fatalf("round trip mismatch for %d byte message", len(msg))
		}
	}
}

func TestSealUsesFreshNonce(t *testing.T) {
	c := testKeys(t).Cipher()
	a, _ := c.Seal([]byte("same"), nil)
	b, _ := c.Seal([]byte("same"), nil)
	if bytes.Equal(a, b) {
		t.Fatal("two encryptions of the same plaintext produced identical ciphertexts")
	}
}

func TestOpenRejectsTampering(t *testing.T) {
	c := testKeys(t).Cipher()
	ct, err := c.Seal([]byte("blood type O-"), []byte("row-1"))
	if err != nil {
		t.Fatal(err)
	}

	flipped := bytes.Clone(ct)
	flipped[len(flipped)-1] ^= 1

	badVersion := bytes.Clone(ct)
	badVersion[0] = 99

	other, _ := NewKeys(bytes.Repeat([]byte{8}, KeySize))

	cases := map[string]func() ([]byte, error){
		"flipped bit":   func() ([]byte, error) { return c.Open(flipped, []byte("row-1")) },
		"wrong aad":     func() ([]byte, error) { return c.Open(ct, []byte("row-2")) },
		"bad version":   func() ([]byte, error) { return c.Open(badVersion, []byte("row-1")) },
		"truncated":     func() ([]byte, error) { return c.Open(ct[:10], []byte("row-1")) },
		"empty":         func() ([]byte, error) { return c.Open(nil, []byte("row-1")) },
		"different key": func() ([]byte, error) { return other.Cipher().Open(ct, []byte("row-1")) },
	}
	for name, open := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := open(); !errors.Is(err, ErrDecrypt) {
				t.Fatalf("err = %v, want ErrDecrypt", err)
			}
		})
	}
}

func TestBlindIndex(t *testing.T) {
	k := testKeys(t)
	a := k.BlindIndex("family", "smith")
	if a != k.BlindIndex("family", "smith") {
		t.Fatal("blind index is not deterministic")
	}
	if a == k.BlindIndex("given", "smith") {
		t.Fatal("blind index does not separate domains")
	}
	if a == k.BlindIndex("family", "smyth") {
		t.Fatal("different values share an index")
	}
	if len(a) != 32 {
		t.Fatalf("index length = %d, want 32 hex chars", len(a))
	}
	other, _ := NewKeys(bytes.Repeat([]byte{9}, KeySize))
	if a == other.BlindIndex("family", "smith") {
		t.Fatal("blind index does not depend on the key")
	}
}

func TestNewKeysRejectsShortMaster(t *testing.T) {
	if _, err := NewKeys([]byte("short")); err == nil {
		t.Fatal("expected error for short master key")
	}
}

func TestNewCipherKeySize(t *testing.T) {
	if _, err := NewCipher(make([]byte, 16)); err == nil {
		t.Fatal("expected error for 16 byte key")
	}
}

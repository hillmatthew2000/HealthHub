package store

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/hillmatthew2000/HealthHub/pkg/encryption"
)

// Blind index domains. Each domain gets its own HMAC input prefix so equal
// values in different fields do not produce equal digests.
const (
	indexFamily    = "patient.family"
	indexBirthDate = "patient.birthdate"
	indexTelecom   = "patient.telecom"
	indexObsCode   = "observation.code"
)

// phi encrypts resources at rest and computes blind indexes over them.
type phi struct {
	keys *encryption.Keys
}

// seal encrypts the JSON encoding of v. The record's table and ID are
// bound as additional data, so a ciphertext copied to another row fails to
// decrypt instead of silently showing another patient's data.
func (p phi) seal(table, id string, v any) ([]byte, error) {
	plain, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("store: encode %s: %w", table, err)
	}
	return p.keys.Cipher().Seal(plain, aad(table, id))
}

// open decrypts a value produced by seal into v.
func (p phi) open(table, id string, data []byte, v any) error {
	plain, err := p.keys.Cipher().Open(data, aad(table, id))
	if err != nil {
		return fmt.Errorf("store: decrypt %s %s: %w", table, id, err)
	}
	if err := json.Unmarshal(plain, v); err != nil {
		return fmt.Errorf("store: decode %s %s: %w", table, id, err)
	}
	return nil
}

func aad(table, id string) []byte { return []byte(table + "/" + id) }

// index returns the blind index of an already normalised value, or "" for
// an empty value so callers can skip it.
func (p phi) index(domain, normalised string) string {
	if normalised == "" {
		return ""
	}
	return p.keys.BlindIndex(domain, normalised)
}

// indexes returns the distinct, non-empty blind indexes of values.
func (p phi) indexes(domain string, normalise func(string) string, values ...string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, v := range values {
		idx := p.index(domain, normalise(v))
		if idx != "" && !seen[idx] {
			seen[idx] = true
			out = append(out, idx)
		}
	}
	return out
}

// NormaliseName folds case and collapses internal whitespace so "  van der
// BERG " and "Van der Berg" index identically.
func NormaliseName(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// NormaliseTelecom normalises a contact value for exact-match lookup:
// phone-like systems keep only digits (and a leading +), everything else
// is lower-cased and trimmed.
func NormaliseTelecom(system, value string) string {
	value = strings.TrimSpace(value)
	switch system {
	case "phone", "fax", "sms", "pager":
		var b strings.Builder
		for i, r := range value {
			if unicode.IsDigit(r) || (r == '+' && i == 0) {
				b.WriteRune(r)
			}
		}
		return b.String()
	default:
		return strings.ToLower(value)
	}
}

// NormaliseCode normalises an observation code ("system|code" or "code").
func NormaliseCode(s string) string {
	return strings.TrimSpace(s)
}

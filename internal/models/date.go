package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// DateLayout is the FHIR date format used for birth dates.
const DateLayout = time.DateOnly

// Date is a calendar date without a time of day, encoded as "YYYY-MM-DD".
type Date struct {
	time.Time
}

// NewDate returns the date of t in UTC.
func NewDate(year int, month time.Month, day int) Date {
	return Date{time.Date(year, month, day, 0, 0, 0, 0, time.UTC)}
}

// ParseDate parses a "YYYY-MM-DD" string.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(DateLayout, s)
	if err != nil {
		return Date{}, fmt.Errorf("date must use the YYYY-MM-DD format")
	}
	return Date{t}, nil
}

// String returns the date as "YYYY-MM-DD".
func (d Date) String() string { return d.Format(DateLayout) }

// MarshalJSON encodes the date as "YYYY-MM-DD".
func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON accepts "YYYY-MM-DD". For compatibility with earlier
// clients an RFC 3339 timestamp is also accepted; its date part is kept.
func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("date must be a string")
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		*d = NewDate(t.Date())
		return nil
	}
	parsed, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

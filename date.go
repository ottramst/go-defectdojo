package defectdojo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// dateLayout is the format of DefectDojo's date fields.
const dateLayout = "2006-01-02"

// Date is a calendar date without a time of day or time zone, such as the
// target start of an engagement. It is sent and read as "2006-01-02". The
// zero Date is sent as null, and null is read as the zero Date.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// DateOf returns the date of t in t's location.
func DateOf(t time.Time) Date {
	y, m, d := t.Date()
	return Date{Year: y, Month: m, Day: d}
}

// ParseDate parses a date in the format "2006-01-02".
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return Date{}, fmt.Errorf("defectdojo: invalid date %q: %w", s, err)
	}
	return DateOf(t), nil
}

// String returns the date in the format "2006-01-02", or "" for the zero
// Date.
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}

// IsZero reports whether d is the zero Date. It makes omitzero work.
func (d Date) IsZero() bool {
	return d == Date{}
}

// MarshalJSON encodes the date as "2006-01-02", or null for the zero Date.
func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.String())
}

// UnmarshalJSON decodes a "2006-01-02" date, or null as the zero Date.
func (d *Date) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*d = Date{}
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("defectdojo: decoding date: %w", err)
	}
	parsed, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

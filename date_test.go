package defectdojo_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ottramst/go-defectdojo"
)

func TestDate_JSON(t *testing.T) {
	tests := []struct {
		date defectdojo.Date
		json string
	}{
		{date: defectdojo.Date{Year: 2026, Month: time.January, Day: 5}, json: `"2026-01-05"`},
		{date: defectdojo.Date{Year: 999, Month: time.December, Day: 31}, json: `"0999-12-31"`},
		{date: defectdojo.Date{}, json: `null`},
	}
	for _, tt := range tests {
		t.Run(tt.json, func(t *testing.T) {
			data, err := json.Marshal(tt.date)
			if err != nil || string(data) != tt.json {
				t.Errorf("Marshal(%v) = %s, %v; want %s", tt.date, data, err, tt.json)
			}
			var got defectdojo.Date
			if err := json.Unmarshal([]byte(tt.json), &got); err != nil || got != tt.date {
				t.Errorf("Unmarshal(%s) = %v, %v; want %v", tt.json, got, err, tt.date)
			}
		})
	}
	for _, bad := range []string{`"2026-1-5"`, `"20260105"`, `"2026-02-30"`, `"2026-01-05T00:00:00Z"`, `5`} {
		var d defectdojo.Date
		if err := json.Unmarshal([]byte(bad), &d); err == nil {
			t.Errorf("Unmarshal(%s) = %v, want an error", bad, d)
		}
	}
}

func TestDate_helpers(t *testing.T) {
	d, err := defectdojo.ParseDate("2026-11-14")
	if err != nil || d != (defectdojo.Date{Year: 2026, Month: time.November, Day: 14}) {
		t.Errorf("ParseDate() = %v, %v", d, err)
	}
	if got := d.String(); got != "2026-11-14" {
		t.Errorf("String() = %q", got)
	}
	if (defectdojo.Date{}).String() != "" || !(defectdojo.Date{}).IsZero() || d.IsZero() {
		t.Error("zero Date: String() or IsZero() wrong")
	}
	loc := time.FixedZone("UTC+14", 14*3600)
	if got := defectdojo.DateOf(time.Date(2026, 11, 14, 23, 0, 0, 0, loc)); got != d {
		t.Errorf("DateOf() = %v, want %v (the date in the time's location)", got, d)
	}
	if _, err := defectdojo.ParseDate("14.11.2026"); err == nil {
		t.Error("ParseDate(14.11.2026) error = nil")
	}
}

// A zero Nullable[Date] is left out of a patch; a zero Date value is null.
func TestDate_inPatch(t *testing.T) {
	data, err := json.Marshal(defectdojo.EngagementPatch{
		TargetEnd:      &defectdojo.Date{Year: 2027, Month: time.March, Day: 1},
		FirstContacted: defectdojo.Null[defectdojo.Date](),
	})
	if err != nil || string(data) != `{"target_end":"2027-03-01","first_contacted":null}` {
		t.Errorf("Marshal() = %s, %v", data, err)
	}
}

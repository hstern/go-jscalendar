// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"encoding/json"
	"strings"
	"testing"
)

// ptrInt builds a pointer to a literal int for the pointer-typed NDay
// NthOfPeriod field, keeping the table entries readable.
func ptrInt(v int) *int { return &v }

// TestRecurrenceRuleRoundTrip exercises decode-then-encode against the
// RRULE-equivalent examples worked through in RFC 8984, Section 4.3, and the
// recurrence examples in Section 6. A rule that decodes from a canonical
// object must re-encode to the same bytes, since the wire form is the
// contract.
func TestRecurrenceRuleRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
	}{
		{
			name: "daily, minimal",
			in:   `{"@type":"RecurrenceRule","frequency":"daily"}`,
		},
		{
			name: "every other week on Monday and Wednesday",
			in: `{"@type":"RecurrenceRule","frequency":"weekly","interval":2,` +
				`"byDay":[{"@type":"NDay","day":"mo"},{"@type":"NDay","day":"we"}]}`,
		},
		{
			name: "monthly on the last Sunday, bounded by count",
			in: `{"@type":"RecurrenceRule","frequency":"monthly",` +
				`"byDay":[{"@type":"NDay","day":"su","nthOfPeriod":-1}],"count":10}`,
		},
		{
			name: "yearly until a local date-time in the event's zone",
			in: `{"@type":"RecurrenceRule","frequency":"yearly",` +
				`"until":"2024-01-15T09:00:00"}`,
		},
		{
			name: "yearly in January and the leap month, by month strings",
			in: `{"@type":"RecurrenceRule","frequency":"yearly",` +
				`"byMonth":["1","5L"]}`,
		},
		{
			name: "complex set-position selection",
			in: `{"@type":"RecurrenceRule","frequency":"monthly",` +
				`"byDay":[{"@type":"NDay","day":"mo"},{"@type":"NDay","day":"tu"},` +
				`{"@type":"NDay","day":"we"},{"@type":"NDay","day":"th"},` +
				`{"@type":"NDay","day":"fr"}],"bySetPosition":[-1]}`,
		},
		{
			name: "every by-filter populated",
			in: `{"@type":"RecurrenceRule","frequency":"secondly","interval":3,` +
				`"rscale":"gregorian","skip":"forward","firstDayOfWeek":"su",` +
				`"byDay":[{"@type":"NDay","day":"we","nthOfPeriod":2}],` +
				`"byMonthDay":[1,-1],"byMonth":["12"],"byYearDay":[100,-1],` +
				`"byWeekNo":[1,-1],"byHour":[9,17],"byMinute":[0,30],` +
				`"bySecond":[0,60],"bySetPosition":[1,-1]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var rule RecurrenceRule
			if err := json.Unmarshal([]byte(tt.in), &rule); err != nil {
				t.Fatalf("Unmarshal(%s) returned error: %v", tt.in, err)
			}

			out, err := json.Marshal(rule)
			if err != nil {
				t.Fatalf("Marshal of decoded rule returned error: %v", err)
			}
			if string(out) != tt.in {
				t.Errorf("round-trip mismatch\n got: %s\nwant: %s", out, tt.in)
			}
		})
	}
}

// TestRecurrenceRuleTypeEmittedFirst checks that "@type" is the first member
// of the marshaled object, the byte-stability contract RFC 8984 Section 1.4.1
// imposes on every object.
func TestRecurrenceRuleTypeEmittedFirst(t *testing.T) {
	t.Parallel()

	out, err := json.Marshal(RecurrenceRule{Frequency: FrequencyDaily})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	const want = `{"@type":"RecurrenceRule","frequency":"daily"}`
	if string(out) != want {
		t.Errorf("Marshal = %s, want %s", out, want)
	}
}

// TestRecurrenceRuleMarshalRequiresFrequency checks that the one structural
// MUST this type can enforce on its own — a present "frequency" — is checked
// at the marshal boundary, while unmarshal stays lenient about its absence.
func TestRecurrenceRuleMarshalRequiresFrequency(t *testing.T) {
	t.Parallel()

	if _, err := json.Marshal(RecurrenceRule{}); err == nil {
		t.Fatal("Marshal of a frequency-less rule succeeded, want error")
	}

	// Unmarshal of an object with no "frequency" must NOT error: decode is
	// lenient, and the missing field surfaces only when re-marshaling.
	var rule RecurrenceRule
	if err := json.Unmarshal([]byte(`{"@type":"RecurrenceRule","interval":2}`), &rule); err != nil {
		t.Fatalf("Unmarshal of a frequency-less object returned error: %v", err)
	}
	if rule.Frequency != "" {
		t.Errorf("Frequency = %q, want empty", rule.Frequency)
	}
	if rule.Interval != 2 {
		t.Errorf("Interval = %d, want 2", rule.Interval)
	}
}

// TestRecurrenceRuleLenientUnmarshal checks the Postel's-law decode posture:
// an unknown frequency, an out-of-range filter value, and a rule carrying
// both "count" and "until" all decode without error so the rule round-trips.
func TestRecurrenceRuleLenientUnmarshal(t *testing.T) {
	t.Parallel()

	const in = `{"@type":"RecurrenceRule","frequency":"fortnightly",` +
		`"byHour":[99],"count":5,"until":"2024-01-01T00:00:00"}`

	var rule RecurrenceRule
	if err := json.Unmarshal([]byte(in), &rule); err != nil {
		t.Fatalf("Unmarshal of a semantically-invalid rule returned error: %v", err)
	}

	if rule.Frequency.IsValid() {
		t.Errorf("Frequency %q reported valid, want invalid", rule.Frequency)
	}
	if !rule.HasCount() || !rule.HasUntil() {
		t.Errorf("HasCount=%v HasUntil=%v, want both true (the conflict the validator flags)",
			rule.HasCount(), rule.HasUntil())
	}
}

// TestRecurrenceRuleUnmarshalNonObject checks that a non-object JSON token is
// rejected, since a RecurrenceRule is always an object on the wire.
func TestRecurrenceRuleUnmarshalNonObject(t *testing.T) {
	t.Parallel()

	for _, in := range []string{`null`, `"daily"`, `[]`, `42`} {
		var rule RecurrenceRule
		if err := json.Unmarshal([]byte(in), &rule); err == nil {
			t.Errorf("Unmarshal(%s) succeeded, want error", in)
		}
	}
}

// TestRecurrenceRuleCountZeroDistinctFromAbsent checks that an explicit
// "count":0 round-trips and is distinguishable from an absent count, which is
// why Count is a pointer.
func TestRecurrenceRuleCountZeroDistinctFromAbsent(t *testing.T) {
	t.Parallel()

	var withZero RecurrenceRule
	if err := json.Unmarshal([]byte(`{"frequency":"daily","count":0}`), &withZero); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if !withZero.HasCount() {
		t.Fatal("HasCount() = false for explicit count:0, want true")
	}
	out, err := json.Marshal(withZero)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if !strings.Contains(string(out), `"count":0`) {
		t.Errorf("Marshal = %s, want it to contain \"count\":0", out)
	}

	var noCount RecurrenceRule
	if err := json.Unmarshal([]byte(`{"frequency":"daily"}`), &noCount); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if noCount.HasCount() {
		t.Error("HasCount() = true for absent count, want false")
	}
}

// TestFrequencyIsValid checks the opt-in frequency check against the seven
// values RFC 8984 Section 4.3.1 defines and a couple it does not.
func TestFrequencyIsValid(t *testing.T) {
	t.Parallel()

	valid := []Frequency{
		FrequencyYearly, FrequencyMonthly, FrequencyWeekly, FrequencyDaily,
		FrequencyHourly, FrequencyMinutely, FrequencySecondly,
	}
	for _, f := range valid {
		if !f.IsValid() {
			t.Errorf("Frequency %q reported invalid, want valid", f)
		}
	}

	for _, f := range []Frequency{"", "fortnightly", "YEARLY", "weekly "} {
		if f.IsValid() {
			t.Errorf("Frequency %q reported valid, want invalid", f)
		}
	}
}

// TestNDayRoundTrip exercises the NDay sub-object both as a bare weekday and
// as an occurrence-pinned one, in both directions.
func TestNDayRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want NDay
	}{
		{
			name: "every Monday",
			in:   `{"@type":"NDay","day":"mo"}`,
			want: NDay{Day: "mo"},
		},
		{
			name: "first Monday of the period",
			in:   `{"@type":"NDay","day":"mo","nthOfPeriod":1}`,
			want: NDay{Day: "mo", NthOfPeriod: ptrInt(1)},
		},
		{
			name: "last Sunday of the period",
			in:   `{"@type":"NDay","day":"su","nthOfPeriod":-1}`,
			want: NDay{Day: "su", NthOfPeriod: ptrInt(-1)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got NDay
			if err := json.Unmarshal([]byte(tt.in), &got); err != nil {
				t.Fatalf("Unmarshal(%s) returned error: %v", tt.in, err)
			}
			if got.Day != tt.want.Day {
				t.Errorf("Day = %q, want %q", got.Day, tt.want.Day)
			}
			switch {
			case got.NthOfPeriod == nil && tt.want.NthOfPeriod != nil,
				got.NthOfPeriod != nil && tt.want.NthOfPeriod == nil:
				t.Errorf("NthOfPeriod presence mismatch: got %v, want %v",
					got.NthOfPeriod, tt.want.NthOfPeriod)
			case got.NthOfPeriod != nil && *got.NthOfPeriod != *tt.want.NthOfPeriod:
				t.Errorf("NthOfPeriod = %d, want %d", *got.NthOfPeriod, *tt.want.NthOfPeriod)
			}

			out, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("Marshal returned error: %v", err)
			}
			if string(out) != tt.in {
				t.Errorf("round-trip mismatch\n got: %s\nwant: %s", out, tt.in)
			}
		})
	}
}

// TestNDayMarshalRequiresDay checks that the mandatory "day" member is
// enforced at the marshal boundary while unmarshal tolerates its absence.
func TestNDayMarshalRequiresDay(t *testing.T) {
	t.Parallel()

	if _, err := json.Marshal(NDay{}); err == nil {
		t.Fatal("Marshal of a day-less NDay succeeded, want error")
	}

	var n NDay
	if err := json.Unmarshal([]byte(`{"@type":"NDay","nthOfPeriod":1}`), &n); err != nil {
		t.Fatalf("Unmarshal of a day-less NDay returned error: %v", err)
	}
	if n.Day != "" {
		t.Errorf("Day = %q, want empty", n.Day)
	}
}

// TestNDayUnmarshalNonObject checks that a non-object token is rejected.
func TestNDayUnmarshalNonObject(t *testing.T) {
	t.Parallel()

	for _, in := range []string{`null`, `"mo"`, `[]`, `7`} {
		var n NDay
		if err := json.Unmarshal([]byte(in), &n); err == nil {
			t.Errorf("Unmarshal(%s) succeeded, want error", in)
		}
	}
}

// TestRecurrenceRuleUntilIsLocal documents and checks the load-bearing pin
// that "until" is a LocalDateTime — zone-free — and not a UTC instant. A
// value with a trailing "Z" is a UTCDateTime and must be rejected by the
// LocalDateTime grammar, so a rule carrying one fails to decode.
func TestRecurrenceRuleUntilIsLocal(t *testing.T) {
	t.Parallel()

	const local = `{"@type":"RecurrenceRule","frequency":"daily","until":"2024-03-10T02:30:00"}`
	var rule RecurrenceRule
	if err := json.Unmarshal([]byte(local), &rule); err != nil {
		t.Fatalf("Unmarshal of a local until returned error: %v", err)
	}
	if !rule.HasUntil() {
		t.Fatal("HasUntil() = false, want true")
	}
	want := LocalDateTime{Year: 2024, Month: 3, Day: 10, Hour: 2, Minute: 30, Second: 0}
	if *rule.Until != want {
		t.Errorf("Until = %+v, want %+v", *rule.Until, want)
	}

	// A "Z"-suffixed value is a UTCDateTime, not a LocalDateTime, and must
	// not decode into the until field.
	const utc = `{"@type":"RecurrenceRule","frequency":"daily","until":"2024-03-10T02:30:00Z"}`
	var bad RecurrenceRule
	if err := json.Unmarshal([]byte(utc), &bad); err == nil {
		t.Error("Unmarshal of a Z-suffixed until succeeded, want error (until is LocalDateTime, not UTC)")
	}
}

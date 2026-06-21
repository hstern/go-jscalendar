// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

// canonical round-trips: parsing then re-formatting must reproduce the
// input byte-for-byte. Every entry here is a string the spec considers
// canonical output of the formatter.
func canonicalDurations() []string {
	return []string{
		"PT0S",
		"PT1S",
		"PT1M",
		"PT1H",
		"P1D",
		"P1W",
		"P1M",
		"P1Y",
		"PT1H30M",
		"PT2H3M4S",
		"P1DT12H",
		"P1Y2M3DT4H5M6S",
		"P15DT5H20S",
		"P7W",
		"PT0.5S",
		"PT1.25S",
		"P1DT0.001S",
	}
}

func TestParseDurationCanonicalRoundTrip(t *testing.T) {
	for _, s := range canonicalDurations() {
		t.Run(s, func(t *testing.T) {
			d, err := ParseDuration(s)
			if err != nil {
				t.Fatalf("ParseDuration(%q) unexpected error: %v", s, err)
			}
			if got := d.String(); got != s {
				t.Errorf("ParseDuration(%q).String() = %q, want %q", s, got, s)
			}
		})
	}
}

func TestParseSignedDurationCanonicalRoundTrip(t *testing.T) {
	cases := append([]string{}, canonicalDurations()...)
	// SignedDuration additionally accepts a leading minus, except the
	// zero (a negative zero formats as the unsigned zero).
	for _, s := range canonicalDurations() {
		if s == "PT0S" {
			continue
		}
		cases = append(cases, "-"+s)
	}
	for _, s := range cases {
		t.Run(s, func(t *testing.T) {
			d, err := ParseSignedDuration(s)
			if err != nil {
				t.Fatalf("ParseSignedDuration(%q) unexpected error: %v", s, err)
			}
			if got := d.String(); got != s {
				t.Errorf("ParseSignedDuration(%q).String() = %q, want %q", s, got, s)
			}
		})
	}
}

// non-canonical but valid inputs the lenient parser must accept; the
// formatter normalizes them to a canonical form.
func TestParseDurationNormalizes(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"P0D", "PT0S"},        // any zero collapses to the canonical zero
		{"PT0H0M0S", "PT0S"},   // zero in time component
		{"P0W", "PT0S"},        // zero weeks
		{"P0Y0M0D", "PT0S"},    // zero in date component
		{"PT60S", "PT1M"},      // seconds overflow into minutes
		{"PT90M", "PT1H30M"},   // minutes overflow into hours
		{"PT1.500S", "PT1.5S"}, // trailing fractional zeros trimmed
		{"PT1.0S", "PT1S"},     // a whole fraction drops the decimal
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			d, err := ParseDuration(c.in)
			if err != nil {
				t.Fatalf("ParseDuration(%q) unexpected error: %v", c.in, err)
			}
			if got := d.String(); got != c.want {
				t.Errorf("ParseDuration(%q).String() = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestParseDurationRejectsMalformed(t *testing.T) {
	bad := []string{
		"",                        // empty
		"P",                       // no components
		"T1H",                     // missing leading P
		"PT",                      // T with no time component
		"1H",                      // no P, no T
		"P1H",                     // hour without the T separator
		"PT1D",                    // day in the time part
		"P1S",                     // second in the date part
		"P1W1D",                   // week cannot combine with other units
		"P1DT1H1W",                // week mixed into a date+time form
		"P1WT1H",                  // week with a time component
		"PT1M1H",                  // components out of order (hour after minute)
		"P1D1Y",                   // date components out of order (year after day)
		"-PT1S",                   // Duration is non-negative; sign is SignedDuration
		"+PT1S",                   // leading plus is never valid
		"PT1.S",                   // fractional point with no digits
		"PT.5S",                   // fractional with no integer part
		"PT1,5S",                  // comma decimal separator not accepted
		"PT1H ",                   // trailing whitespace
		" PT1H",                   // leading whitespace
		"pt1h",                    // lowercase
		"P1.5D",                   // fraction only allowed on seconds
		"PT1.5H",                  // fraction only allowed on seconds
		"PT1H30",                  // dangling number with no unit
		"P-1D",                    // sign inside a component
		"PT1H-30M",                // sign inside a component
		"PnW",                     // literal grammar text, not a value
		"PT0.S",                   // empty fraction
		"P1Y2M3DT",                // trailing T with no time component
		"PT99999999999999999999H", // hours magnitude exceeds uint64
		"PT18446744073709551615H", // hours alone overflow once scaled to seconds
	}
	for _, s := range bad {
		t.Run(s, func(t *testing.T) {
			if d, err := ParseDuration(s); err == nil {
				t.Errorf("ParseDuration(%q) = %v, want error", s, d)
			}
		})
	}
}

func TestParseSignedDurationRejectsMalformed(t *testing.T) {
	bad := []string{
		"",
		"P",
		"-",
		"-P",
		"--PT1S",
		"+PT1S",
		"-T1H",
		"- PT1S",
		"-P1W1D",
		"PT1H-30M",
	}
	for _, s := range bad {
		t.Run(s, func(t *testing.T) {
			if d, err := ParseSignedDuration(s); err == nil {
				t.Errorf("ParseSignedDuration(%q) = %v, want error", s, d)
			}
		})
	}
}

func TestDurationZeroValue(t *testing.T) {
	var d Duration
	if got := d.String(); got != "PT0S" {
		t.Errorf("zero Duration.String() = %q, want %q", got, "PT0S")
	}
	var s SignedDuration
	if got := s.String(); got != "PT0S" {
		t.Errorf("zero SignedDuration.String() = %q, want %q", got, "PT0S")
	}
	if !d.IsZero() {
		t.Error("zero Duration.IsZero() = false, want true")
	}
}

func TestSignedDurationNegative(t *testing.T) {
	d, err := ParseSignedDuration("-PT1H")
	if err != nil {
		t.Fatalf("ParseSignedDuration(-PT1H) error: %v", err)
	}
	if !d.Negative {
		t.Error("Negative = false, want true")
	}
	if got := d.String(); got != "-PT1H" {
		t.Errorf("String() = %q, want %q", got, "-PT1H")
	}
}

func TestDurationJSONRoundTrip(t *testing.T) {
	for _, s := range canonicalDurations() {
		var d Duration
		quoted := `"` + s + `"`
		if err := json.Unmarshal([]byte(quoted), &d); err != nil {
			t.Errorf("Unmarshal(%s) error: %v", quoted, err)
			continue
		}
		out, err := json.Marshal(d)
		if err != nil {
			t.Errorf("Marshal(%q) error: %v", s, err)
			continue
		}
		if string(out) != quoted {
			t.Errorf("JSON round-trip of %s = %s", quoted, out)
		}
	}
}

func TestSignedDurationJSONRoundTrip(t *testing.T) {
	quoted := `"-PT30M"`
	var d SignedDuration
	if err := json.Unmarshal([]byte(quoted), &d); err != nil {
		t.Fatalf("Unmarshal(%s) error: %v", quoted, err)
	}
	out, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}
	if string(out) != quoted {
		t.Errorf("JSON round-trip = %s, want %s", out, quoted)
	}
}

func TestDurationUnmarshalRejectsNonString(t *testing.T) {
	var d Duration
	if err := json.Unmarshal([]byte(`123`), &d); err == nil {
		t.Error("Unmarshal(123) into Duration: want error")
	}
}

func TestDurationUnmarshalRejectsSigned(t *testing.T) {
	var d Duration
	if err := json.Unmarshal([]byte(`"-PT1H"`), &d); err == nil {
		t.Error("Unmarshal(-PT1H) into Duration: want error (non-negative type)")
	}
}

func TestDurationMarshalText(t *testing.T) {
	d, err := ParseDuration("PT1H30M")
	if err != nil {
		t.Fatal(err)
	}
	b, err := d.MarshalText()
	if err != nil {
		t.Fatalf("MarshalText error: %v", err)
	}
	if string(b) != "PT1H30M" {
		t.Errorf("MarshalText = %q, want %q", b, "PT1H30M")
	}
	var got Duration
	if err := got.UnmarshalText([]byte("PT1H30M")); err != nil {
		t.Fatalf("UnmarshalText error: %v", err)
	}
	if got.String() != "PT1H30M" {
		t.Errorf("UnmarshalText round-trip = %q", got.String())
	}
}

func TestParseDurationErrorIsSentinel(t *testing.T) {
	_, err := ParseDuration("nope")
	if !errors.Is(err, ErrInvalidDuration) {
		t.Errorf("ParseDuration error = %v, want wrapping ErrInvalidDuration", err)
	}
}

// the week form and the calendar form are mutually exclusive; a parsed
// week duration reports its weeks and nothing else.
func TestDurationWeeks(t *testing.T) {
	d, err := ParseDuration("P3W")
	if err != nil {
		t.Fatal(err)
	}
	if d.Weeks != 3 {
		t.Errorf("Weeks = %d, want 3", d.Weeks)
	}
	if d.Days != 0 || d.Hours != 0 {
		t.Errorf("week form leaked into other fields: %+v", d)
	}
}

// a large but representable time part must parse and normalize without
// tripping the overflow guard. 1000000H = 1000000*3600 s, well within uint64.
func TestParseDurationLargeButRepresentable(t *testing.T) {
	d, err := ParseDuration("PT1000000H")
	if err != nil {
		t.Fatalf("ParseDuration(PT1000000H) error: %v", err)
	}
	if got := d.String(); got != "PT1000000H" {
		t.Errorf("String() = %q, want %q", got, "PT1000000H")
	}
}

func ExampleParseDuration() {
	d, err := ParseDuration("PT1H30M")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Printf("%d hours, %d minutes -> %s\n", d.Hours, d.Minutes, d)
	// Output: 1 hours, 30 minutes -> PT1H30M
}

func ExampleParseDuration_normalizes() {
	// The parser is lenient: non-canonical-but-valid input is accepted and
	// re-formatted to canonical form. Here 90 seconds becomes a minute and
	// a half.
	d, _ := ParseDuration("PT90S")
	fmt.Println(d)
	// Output: PT1M30S
}

func ExampleParseSignedDuration() {
	// SignedDuration permits a leading minus, as used by alert offsets that
	// fire before their anchor.
	d, _ := ParseSignedDuration("-PT15M")
	fmt.Printf("negative=%t value=%s\n", d.Negative, d)
	// Output: negative=true value=-PT15M
}

// FuzzParseDuration asserts two invariants for the hand-rolled grammar: the
// parser never panics on arbitrary input, and the formatter is a fixpoint —
// re-parsing the canonical text of any accepted value succeeds and yields
// the identical string. A formatter that emitted text the parser rejected,
// or a parse/format asymmetry, would be caught here rather than only by the
// fixed corpus above.
func FuzzParseDuration(f *testing.F) {
	for _, s := range canonicalDurations() {
		f.Add(s)
	}
	f.Add("P0D")
	f.Add("garbage")
	f.Add("")

	f.Fuzz(func(t *testing.T, s string) {
		d, err := ParseDuration(s)
		if err != nil {
			return // rejection is a valid outcome; only panics are bugs
		}
		canon := d.String()
		again, err := ParseDuration(canon)
		if err != nil {
			t.Fatalf("ParseDuration(%q) accepted but its canonical form %q was rejected: %v", s, canon, err)
		}
		if got := again.String(); got != canon {
			t.Fatalf("formatter is not a fixpoint: %q -> %q -> %q", s, canon, got)
		}
	})
}

// FuzzParseSignedDuration mirrors FuzzParseDuration for the signed grammar.
func FuzzParseSignedDuration(f *testing.F) {
	for _, s := range canonicalDurations() {
		f.Add(s)
		f.Add("-" + s)
	}
	f.Add("-garbage")
	f.Add("-")

	f.Fuzz(func(t *testing.T, s string) {
		d, err := ParseSignedDuration(s)
		if err != nil {
			return
		}
		canon := d.String()
		again, err := ParseSignedDuration(canon)
		if err != nil {
			t.Fatalf("ParseSignedDuration(%q) accepted but its canonical form %q was rejected: %v", s, canon, err)
		}
		if got := again.String(); got != canon {
			t.Fatalf("formatter is not a fixpoint: %q -> %q -> %q", s, canon, got)
		}
	})
}

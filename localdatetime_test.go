// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestLocalDateTimeUnmarshalValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want LocalDateTime
	}{
		{
			name: "spec example midday",
			in:   `"2020-01-15T13:00:00"`,
			want: LocalDateTime{Year: 2020, Month: 1, Day: 15, Hour: 13, Minute: 0, Second: 0},
		},
		{
			name: "midnight",
			in:   `"2020-01-01T00:00:00"`,
			want: LocalDateTime{Year: 2020, Month: 1, Day: 1, Hour: 0, Minute: 0, Second: 0},
		},
		{
			name: "end of day",
			in:   `"2021-12-31T23:59:59"`,
			want: LocalDateTime{Year: 2021, Month: 12, Day: 31, Hour: 23, Minute: 59, Second: 59},
		},
		{
			name: "leap second component value 60 is rejected elsewhere but 59 ok",
			in:   `"2024-02-29T08:30:45"`,
			want: LocalDateTime{Year: 2024, Month: 2, Day: 29, Hour: 8, Minute: 30, Second: 45},
		},
		{
			name: "year zero-padded four digits",
			in:   `"0001-01-01T00:00:00"`,
			want: LocalDateTime{Year: 1, Month: 1, Day: 1, Hour: 0, Minute: 0, Second: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got LocalDateTime
			if err := json.Unmarshal([]byte(tt.in), &got); err != nil {
				t.Fatalf("Unmarshal(%s) returned error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("Unmarshal(%s) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestLocalDateTimeUnmarshalRejects(t *testing.T) {
	t.Parallel()

	// Each input is a JSON document that MUST be rejected because it does
	// not match the §1.4.4 grammar: YYYY-MM-DDTHH:MM:SS with no offset,
	// no trailing "Z", and no fractional seconds.
	tests := []struct {
		name string
		in   string
	}{
		{"trailing Z (that is UTCDateTime)", `"2020-01-15T13:00:00Z"`},
		{"numeric offset", `"2020-01-15T13:00:00+05:00"`},
		{"negative offset", `"2020-01-15T13:00:00-08:00"`},
		{"fractional seconds", `"2020-01-15T13:00:00.500"`},
		{"fractional seconds with Z", `"2020-01-15T13:00:00.500Z"`},
		{"missing seconds", `"2020-01-15T13:00"`},
		{"missing time", `"2020-01-15"`},
		{"space separator instead of T", `"2020-01-15 13:00:00"`},
		{"lowercase t separator", `"2020-01-15t13:00:00"`},
		{"two-digit year", `"20-01-15T13:00:00"`},
		{"month 13", `"2020-13-15T13:00:00"`},
		{"month 00", `"2020-00-15T13:00:00"`},
		{"day 32", `"2020-01-32T13:00:00"`},
		{"day 00", `"2020-01-00T13:00:00"`},
		{"hour 24", `"2020-01-15T24:00:00"`},
		{"minute 60", `"2020-01-15T13:60:00"`},
		{"second 60", `"2020-01-15T13:00:60"`},
		{"feb 30 invalid calendar day", `"2021-02-30T00:00:00"`},
		{"non-leap feb 29", `"2021-02-29T00:00:00"`},
		{"apr 31 invalid", `"2021-04-31T00:00:00"`},
		{"single-digit month not zero padded", `"2020-1-15T13:00:00"`},
		{"single-digit hour not zero padded", `"2020-01-15T3:00:00"`},
		{"empty string", `""`},
		{"trailing garbage", `"2020-01-15T13:00:00x"`},
		{"leading whitespace", `" 2020-01-15T13:00:00"`},
		{"json number not string", `12345`},
		{"json null", `null`},
		{"json object", `{}`},
		{"signed year field", `"+020-01-15T13:00:00"`},
		{"signed month field", `"2020-+1-15T13:00:00"`},
		{"multibyte rune shifts byte length", `"2020-01-15T13:00:0é"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got LocalDateTime
			if err := json.Unmarshal([]byte(tt.in), &got); err == nil {
				t.Errorf("Unmarshal(%s) = %+v, want error", tt.in, got)
			}
		})
	}
}

func TestLocalDateTimeMarshalRoundTrip(t *testing.T) {
	t.Parallel()

	// Byte-stability: marshaling a value parsed from a wire string must
	// reproduce the exact same bytes.
	inputs := []string{
		`"2020-01-15T13:00:00"`,
		`"0001-01-01T00:00:00"`,
		`"2021-12-31T23:59:59"`,
		`"2024-02-29T08:30:45"`,
		`"2020-01-01T00:00:00"`,
	}

	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			t.Parallel()

			var v LocalDateTime
			if err := json.Unmarshal([]byte(in), &v); err != nil {
				t.Fatalf("Unmarshal(%s): %v", in, err)
			}
			out, err := json.Marshal(v)
			if err != nil {
				t.Fatalf("Marshal(%+v): %v", v, err)
			}
			if string(out) != in {
				t.Errorf("round-trip: Marshal(Unmarshal(%s)) = %s, want %s", in, out, in)
			}
		})
	}
}

func TestLocalDateTimeMarshalZeroPads(t *testing.T) {
	t.Parallel()

	v := LocalDateTime{Year: 5, Month: 3, Day: 7, Hour: 9, Minute: 4, Second: 2}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	const want = `"0005-03-07T09:04:02"`
	if string(out) != want {
		t.Errorf("Marshal(%+v) = %s, want %s", v, out, want)
	}
}

func TestLocalDateTimeMarshalRejectsOutOfRange(t *testing.T) {
	t.Parallel()

	// A LocalDateTime constructed in Go code with out-of-range fields is
	// rejected at the marshal boundary (strict marshal), since the wire
	// form would otherwise be non-conformant.
	tests := []struct {
		name string
		in   LocalDateTime
	}{
		{"month 13", LocalDateTime{Year: 2020, Month: 13, Day: 1}},
		{"day 0", LocalDateTime{Year: 2020, Month: 1, Day: 0}},
		{"feb 30", LocalDateTime{Year: 2020, Month: 2, Day: 30}},
		{"hour 24", LocalDateTime{Year: 2020, Month: 1, Day: 1, Hour: 24}},
		{"negative year", LocalDateTime{Year: -1, Month: 1, Day: 1}},
		{"year over 9999", LocalDateTime{Year: 10000, Month: 1, Day: 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if out, err := json.Marshal(tt.in); err == nil {
				t.Errorf("Marshal(%+v) = %s, want error", tt.in, out)
			}
		})
	}
}

func TestLocalDateTimeString(t *testing.T) {
	t.Parallel()

	v := LocalDateTime{Year: 2020, Month: 1, Day: 15, Hour: 13, Minute: 0, Second: 0}
	if got := v.String(); got != "2020-01-15T13:00:00" {
		t.Errorf("String() = %q, want %q", got, "2020-01-15T13:00:00")
	}
}

func TestParseLocalDateTime(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()

		got, err := ParseLocalDateTime("2020-01-15T13:00:00")
		if err != nil {
			t.Fatalf("ParseLocalDateTime: %v", err)
		}
		want := LocalDateTime{Year: 2020, Month: 1, Day: 15, Hour: 13}
		if got != want {
			t.Errorf("ParseLocalDateTime = %+v, want %+v", got, want)
		}
	})

	t.Run("rejects trailing Z", func(t *testing.T) {
		t.Parallel()

		if _, err := ParseLocalDateTime("2020-01-15T13:00:00Z"); err == nil {
			t.Error("ParseLocalDateTime accepted a UTCDateTime string, want error")
		}
	})
}

func ExampleLocalDateTime() {
	// A LocalDateTime carries no zone; the wire form has no trailing "Z".
	start := LocalDateTime{Year: 2020, Month: 1, Day: 15, Hour: 13}
	b, err := json.Marshal(start)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(b))
	// Output: "2020-01-15T13:00:00"
}

func ExampleParseLocalDateTime() {
	lt, err := ParseLocalDateTime("2020-01-15T13:00:00")
	if err != nil {
		panic(err)
	}
	fmt.Printf("%d-%02d-%02d at %02d:00\n", lt.Year, lt.Month, lt.Day, lt.Hour)
	// Output: 2020-01-15 at 13:00
}

// FuzzLocalDateTime asserts two invariants over arbitrary input: any
// string the parser accepts must marshal back to the identical bytes
// (no parse/format asymmetry), and a string accepted as a LocalDateTime
// must never also be accepted as a UTCDateTime — the trailing-"Z"
// distinction must partition the two grammars.
func FuzzLocalDateTime(f *testing.F) {
	for _, seed := range []string{
		"2020-01-15T13:00:00",
		"2020-01-15T13:00:00Z",
		"0001-01-01T00:00:00",
		"2021-02-29T00:00:00",
		"not a date",
		"",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		lt, err := ParseLocalDateTime(raw)
		if err != nil {
			return
		}

		out, marshalErr := json.Marshal(lt)
		if marshalErr != nil {
			t.Fatalf("ParseLocalDateTime accepted %q but Marshal rejected it: %v", raw, marshalErr)
		}
		want := `"` + raw + `"`
		if string(out) != want {
			t.Errorf("round-trip: Marshal(ParseLocalDateTime(%q)) = %s, want %s", raw, out, want)
		}

		if _, utcErr := ParseUTCDateTime(raw); utcErr == nil {
			t.Errorf("%q parsed as BOTH LocalDateTime and UTCDateTime; the two grammars must be disjoint", raw)
		}
	})
}

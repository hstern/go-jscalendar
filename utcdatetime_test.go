// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestUTCDateTimeUnmarshalValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want UTCDateTime
	}{
		{
			name: "spec example",
			in:   `"2020-01-02T18:23:04Z"`,
			want: UTCDateTime{Year: 2020, Month: 1, Day: 2, Hour: 18, Minute: 23, Second: 4},
		},
		{
			name: "midnight",
			in:   `"2020-01-01T00:00:00Z"`,
			want: UTCDateTime{Year: 2020, Month: 1, Day: 1},
		},
		{
			name: "end of day",
			in:   `"2021-12-31T23:59:59Z"`,
			want: UTCDateTime{Year: 2021, Month: 12, Day: 31, Hour: 23, Minute: 59, Second: 59},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got UTCDateTime
			if err := json.Unmarshal([]byte(tt.in), &got); err != nil {
				t.Fatalf("Unmarshal(%s) returned error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("Unmarshal(%s) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestUTCDateTimeUnmarshalRejects(t *testing.T) {
	t.Parallel()

	// §1.4.5: same grammar as LocalDateTime but MUST end in "Z". Anything
	// without the trailing Z, or with an offset, or with fractional
	// seconds, is rejected.
	tests := []struct {
		name string
		in   string
	}{
		{"missing trailing Z (that is LocalDateTime)", `"2020-01-02T18:23:04"`},
		{"lowercase z", `"2020-01-02T18:23:04z"`},
		{"numeric offset instead of Z", `"2020-01-02T18:23:04+00:00"`},
		{"zero offset spelled out", `"2020-01-02T18:23:04-00:00"`},
		{"fractional seconds with Z", `"2020-01-02T18:23:04.5Z"`},
		{"fractional seconds no Z", `"2020-01-02T18:23:04.5"`},
		{"double Z", `"2020-01-02T18:23:04ZZ"`},
		{"Z then garbage", `"2020-01-02T18:23:04Zx"`},
		{"missing seconds", `"2020-01-02T18:23Z"`},
		{"date only with Z", `"2020-01-02Z"`},
		{"month 13", `"2020-13-02T18:23:04Z"`},
		{"day 00", `"2020-01-00T18:23:04Z"`},
		{"hour 24", `"2020-01-02T24:23:04Z"`},
		{"minute 60", `"2020-01-02T18:60:04Z"`},
		{"second 60", `"2020-01-02T18:23:60Z"`},
		{"non-leap feb 29 with Z", `"2021-02-29T00:00:00Z"`},
		{"empty string", `""`},
		{"json number", `12345`},
		{"json null", `null`},
		{"json object", `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got UTCDateTime
			if err := json.Unmarshal([]byte(tt.in), &got); err == nil {
				t.Errorf("Unmarshal(%s) = %+v, want error", tt.in, got)
			}
		})
	}
}

func TestUTCDateTimeMarshalRoundTrip(t *testing.T) {
	t.Parallel()

	inputs := []string{
		`"2020-01-02T18:23:04Z"`,
		`"0001-01-01T00:00:00Z"`,
		`"2021-12-31T23:59:59Z"`,
		`"2020-01-01T00:00:00Z"`,
	}

	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			t.Parallel()

			var v UTCDateTime
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

func TestUTCDateTimeMarshalAppendsZ(t *testing.T) {
	t.Parallel()

	v := UTCDateTime{Year: 2020, Month: 1, Day: 2, Hour: 18, Minute: 23, Second: 4}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	const want = `"2020-01-02T18:23:04Z"`
	if string(out) != want {
		t.Errorf("Marshal(%+v) = %s, want %s", v, out, want)
	}
}

func TestUTCDateTimeMarshalRejectsOutOfRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   UTCDateTime
	}{
		{"month 13", UTCDateTime{Year: 2020, Month: 13, Day: 1}},
		{"day 0", UTCDateTime{Year: 2020, Month: 1, Day: 0}},
		{"feb 30", UTCDateTime{Year: 2020, Month: 2, Day: 30}},
		{"hour 24", UTCDateTime{Year: 2020, Month: 1, Day: 1, Hour: 24}},
		{"year over 9999", UTCDateTime{Year: 10000, Month: 1, Day: 1}},
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

func TestUTCDateTimeString(t *testing.T) {
	t.Parallel()

	v := UTCDateTime{Year: 2020, Month: 1, Day: 2, Hour: 18, Minute: 23, Second: 4}
	if got := v.String(); got != "2020-01-02T18:23:04Z" {
		t.Errorf("String() = %q, want %q", got, "2020-01-02T18:23:04Z")
	}
}

func TestParseUTCDateTime(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()

		got, err := ParseUTCDateTime("2020-01-02T18:23:04Z")
		if err != nil {
			t.Fatalf("ParseUTCDateTime: %v", err)
		}
		want := UTCDateTime{Year: 2020, Month: 1, Day: 2, Hour: 18, Minute: 23, Second: 4}
		if got != want {
			t.Errorf("ParseUTCDateTime = %+v, want %+v", got, want)
		}
	})

	t.Run("rejects missing Z", func(t *testing.T) {
		t.Parallel()

		if _, err := ParseUTCDateTime("2020-01-02T18:23:04"); err == nil {
			t.Error("ParseUTCDateTime accepted a LocalDateTime string, want error")
		}
	})
}

// TestUTCDateTimeToFromTime exercises the time.Time bridge: UTCDateTime,
// unlike LocalDateTime, denotes an absolute instant and can convert.
func TestUTCDateTimeToFromTime(t *testing.T) {
	t.Parallel()

	v := UTCDateTime{Year: 2020, Month: 1, Day: 2, Hour: 18, Minute: 23, Second: 4}
	tm := v.Time()
	if tm.Location() != time.UTC {
		t.Errorf("Time().Location() = %v, want UTC", tm.Location())
	}
	got := UTCDateTimeFromTime(tm)
	if got != v {
		t.Errorf("UTCDateTimeFromTime(Time()) = %+v, want %+v", got, v)
	}
}

func ExampleUTCDateTime() {
	// A UTCDateTime always renders with a trailing "Z".
	updated := UTCDateTime{Year: 2020, Month: 1, Day: 2, Hour: 18, Minute: 23, Second: 4}
	b, err := json.Marshal(updated)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(b))
	// Output: "2020-01-02T18:23:04Z"
}

func ExampleUTCDateTimeFromTime() {
	// Any time.Time is normalized to UTC before its fields are read.
	loc := time.FixedZone("EST", -5*60*60)
	t := time.Date(2020, time.January, 2, 13, 23, 4, 0, loc)
	fmt.Println(UTCDateTimeFromTime(t))
	// Output: 2020-01-02T18:23:04Z
}

// FuzzUTCDateTime asserts that any string the parser accepts marshals
// back to the identical bytes, and that an accepted UTCDateTime is never
// also a valid LocalDateTime (the mandatory trailing "Z" must keep the
// grammars disjoint).
func FuzzUTCDateTime(f *testing.F) {
	for _, seed := range []string{
		"2020-01-02T18:23:04Z",
		"2020-01-02T18:23:04",
		"0001-01-01T00:00:00Z",
		"2021-02-29T00:00:00Z",
		"not a date",
		"",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		ut, err := ParseUTCDateTime(raw)
		if err != nil {
			return
		}

		out, marshalErr := json.Marshal(ut)
		if marshalErr != nil {
			t.Fatalf("ParseUTCDateTime accepted %q but Marshal rejected it: %v", raw, marshalErr)
		}
		want := `"` + raw + `"`
		if string(out) != want {
			t.Errorf("round-trip: Marshal(ParseUTCDateTime(%q)) = %s, want %s", raw, out, want)
		}

		if _, localErr := ParseLocalDateTime(raw); localErr == nil {
			t.Errorf("%q parsed as BOTH UTCDateTime and LocalDateTime; the two grammars must be disjoint", raw)
		}
	})
}

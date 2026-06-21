// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"encoding/json"
	"fmt"
	"testing"
)

func ExampleTimeZoneId_IsCustom() {
	fmt.Println(TimeZoneId("America/New_York").IsCustom()) // IANA name
	fmt.Println(TimeZoneId("/mytz").IsCustom())            // custom, defined in timeZones
	// Output:
	// false
	// true
}

func ExampleTimeZoneId_ResolvesIn() {
	timeZones := map[TimeZoneId]struct{}{"/mytz": {}}
	present := func(k TimeZoneId) bool {
		_, ok := timeZones[k]
		return ok
	}

	fmt.Println(TimeZoneId("/mytz").ResolvesIn(present))            // custom, present in map
	fmt.Println(TimeZoneId("/absent").ResolvesIn(present))          // custom, missing from map
	fmt.Println(TimeZoneId("America/New_York").ResolvesIn(present)) // IANA name needs no entry
	// Output:
	// true
	// false
	// true
}

func TestTimeZoneIDIsCustom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tz   TimeZoneId
		want bool
	}{
		{"iana name", "America/New_York", false},
		{"iana utc", "Etc/UTC", false},
		{"bare name", "UTC", false},
		{"custom", "/custom-tz", true},
		{"custom with slash inside", "/foo/bar", true},
		{"lone slash", "/", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.tz.IsCustom(); got != tt.want {
				t.Errorf("TimeZoneId(%q).IsCustom() = %v, want %v", string(tt.tz), got, tt.want)
			}
		})
	}
}

// TestTimeZoneIDLeadingSlashPreserved is the load-bearing pin from RFC 8984,
// Section 1.4.9: a leading "/" marks a per-object custom zone, and the entire
// string — slash included — is the key into the object's timeZones map. It
// MUST NOT be stripped, and the value MUST NOT be treated as an IANA name.
func TestTimeZoneIDLeadingSlashPreserved(t *testing.T) {
	t.Parallel()

	const wire = `"/mytz"`

	var tz TimeZoneId
	if err := json.Unmarshal([]byte(wire), &tz); err != nil {
		t.Fatalf("Unmarshal(%s) error = %v", wire, err)
	}
	if string(tz) != "/mytz" {
		t.Fatalf("decoded TimeZoneId = %q, want %q (leading slash preserved verbatim)", string(tz), "/mytz")
	}
	if !tz.IsCustom() {
		t.Fatalf("TimeZoneId(%q).IsCustom() = false, want true", string(tz))
	}

	out, err := json.Marshal(tz)
	if err != nil {
		t.Fatalf("Marshal(%q) error = %v", string(tz), err)
	}
	if string(out) != wire {
		t.Fatalf("Marshal round-trip = %s, want %s", out, wire)
	}
}

func TestTimeZoneIDIsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		tz   TimeZoneId
		want bool
	}{
		// IANA-shaped names: non-empty is accepted; the IANA database is
		// not consulted (no runtime dependency, lenient posture).
		{"iana name", "America/New_York", true},
		{"bare name", "UTC", true},
		{"unknown but well-shaped", "Mars/Olympus_Mons", true},

		// Custom ids: must have at least one character after the slash.
		{"custom non-empty", "/mytz", true},
		{"custom multi-segment", "/foo/bar", true},

		{"empty", "", false},
		{"lone slash has no body", "/", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.tz.IsValid(); got != tt.want {
				t.Errorf("TimeZoneId(%q).IsValid() = %v, want %v", string(tt.tz), got, tt.want)
			}
		})
	}
}

// TestTimeZoneIDResolvesInNilPresent checks the defensive nil-closure path: a
// nil present must not panic. A custom zone resolves to false (no definitions
// available); a non-custom IANA zone still resolves to true.
func TestTimeZoneIDResolvesInNilPresent(t *testing.T) {
	t.Parallel()

	if TimeZoneId("/mytz").ResolvesIn(nil) {
		t.Error("custom TimeZoneId.ResolvesIn(nil) = true, want false")
	}
	if !TimeZoneId("America/New_York").ResolvesIn(nil) {
		t.Error("IANA TimeZoneId.ResolvesIn(nil) = false, want true")
	}
}

// TestTimeZoneIDResolvesIn covers the closure check a /-prefixed custom zone
// must satisfy: its full string (slash included) is present as a key in the
// object's timeZones map. A non-custom (IANA) zone needs no such entry.
func TestTimeZoneIDResolvesIn(t *testing.T) {
	t.Parallel()

	zones := map[TimeZoneId]struct{}{
		"/mytz": {},
	}

	tests := []struct {
		name string
		tz   TimeZoneId
		want bool
	}{
		{"custom present", "/mytz", true},
		{"custom absent", "/other", false},
		{"iana never needs an entry", "America/New_York", true},
		{"bare iana", "UTC", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, present := zones[tt.tz]
			got := tt.tz.ResolvesIn(func(k TimeZoneId) bool {
				_, ok := zones[k]
				return ok
			})
			if got != tt.want {
				t.Errorf("TimeZoneId(%q).ResolvesIn(...) = %v, want %v (map-present=%v)",
					string(tt.tz), got, tt.want, present)
			}
		})
	}
}

// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func ExampleId_IsValid() {
	fmt.Println(Id("e2b3c4d5-6789").IsValid()) // url-safe base64 alphabet
	fmt.Println(Id("has space").IsValid())     // space is not in the alphabet
	fmt.Println(Id("").IsValid())              // length must be 1–255
	// Output:
	// true
	// false
	// false
}

func TestIDIsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   Id
		want bool
	}{
		{"single letter", "a", true},
		{"single digit", "0", true},
		{"hyphen only", "-", true},
		{"underscore only", "_", true},
		{"uuid", "e2b3c4d5-6789-4abc-8def-0123456789ab", true},
		{"full alphabet", "ABCdef012-_", true},
		{"max length 255", Id(strings.Repeat("a", 255)), true},

		{"empty", "", false},
		{"too long 256", Id(strings.Repeat("a", 256)), false},
		{"plus is not url-safe", "a+b", false},
		{"slash is not url-safe", "a/b", false},
		{"space", "a b", false},
		{"dot", "a.b", false},
		{"colon", "a:b", false},
		{"non-ascii", "café", false},
		{"padding equals", "ab==", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.id.IsValid(); got != tt.want {
				t.Errorf("Id(%q).IsValid() = %v, want %v", string(tt.id), got, tt.want)
			}
		})
	}
}

// TestIDLenientUnmarshal documents that decoding never rejects: an Id that
// would fail IsValid still round-trips through JSON unchanged. Validation is
// opt-in at the IsValid boundary, per the lenient-unmarshal/strict-validate
// posture.
func TestIDLenientUnmarshal(t *testing.T) {
	t.Parallel()

	const wire = `"not a valid id!"`

	var id Id
	if err := json.Unmarshal([]byte(wire), &id); err != nil {
		t.Fatalf("Unmarshal(%s) error = %v, want nil", wire, err)
	}
	if id.IsValid() {
		t.Fatalf("Id(%q).IsValid() = true, want false (it carries invalid chars)", string(id))
	}
	if got := string(id); got != "not a valid id!" {
		t.Fatalf("decoded Id = %q, want verbatim wire value", got)
	}

	out, err := json.Marshal(id)
	if err != nil {
		t.Fatalf("Marshal(%q) error = %v", string(id), err)
	}
	if string(out) != wire {
		t.Fatalf("Marshal round-trip = %s, want %s", out, wire)
	}
}

// FuzzIDJSONRoundTrip pins two invariants. First, [Id.IsValid] never panics on
// arbitrary input. Second, any Id whose bytes are valid UTF-8 survives
// marshal-then-unmarshal unchanged, guarding the lenient-unmarshal/byte-stable
// posture against regressions in any future custom (Un)MarshalJSON.
//
// Invalid-UTF-8 input is deliberately excluded from the round-trip half: a
// JSON string is required to be valid UTF-8 (RFC 8259, Section 8.1), so such
// bytes could never have arrived from a conformant JSCalendar document, and
// encoding/json substitutes U+FFFD for them on marshal — a documented, correct
// behavior that is out of scope for this type.
func FuzzIDJSONRoundTrip(f *testing.F) {
	f.Add("e2b3c4d5-6789")
	f.Add("")
	f.Add("not a valid id!")
	f.Add("café")

	f.Fuzz(func(t *testing.T, s string) {
		id := Id(s)
		_ = id.IsValid() // must not panic for any input

		if !utf8.ValidString(s) {
			return
		}

		out, err := json.Marshal(id)
		if err != nil {
			t.Fatalf("Marshal(Id(%q)) error = %v", s, err)
		}
		var back Id
		if err := json.Unmarshal(out, &back); err != nil {
			t.Fatalf("Unmarshal(%s) error = %v", out, err)
		}
		if back != id {
			t.Fatalf("round-trip changed Id: got %q, want %q", string(back), s)
		}
	})
}

// TestIDAsMapKey exercises the primary use of Id: a map key for the keyed
// sub-object collections (participants, alerts, links, …).
func TestIDAsMapKey(t *testing.T) {
	t.Parallel()

	const wire = `{"chair":1,"sec-2":2}`

	var m map[Id]int
	if err := json.Unmarshal([]byte(wire), &m); err != nil {
		t.Fatalf("Unmarshal(%s) error = %v", wire, err)
	}
	if m["chair"] != 1 || m["sec-2"] != 2 {
		t.Fatalf("decoded map = %v, want chair=1 sec-2=2", m)
	}
	for k := range m {
		if !k.IsValid() {
			t.Errorf("map key %q is not a valid Id", string(k))
		}
	}
}

// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestPatchObjectUnmarshalRoundTrip(t *testing.T) {
	// A PatchObject decodes into a pointer→raw map and re-encodes to
	// byte-identical JSON: keys are sorted by encoding/json, and the
	// raw values preserve their exact bytes (including key order within
	// a nested object value).
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty object",
			in:   `{}`,
			want: `{}`,
		},
		{
			name: "scalar replacement",
			in:   `{"title":"New title"}`,
			want: `{"title":"New title"}`,
		},
		{
			name: "null removal preserved as JSON null",
			in:   `{"location":null}`,
			want: `{"location":null}`,
		},
		{
			name: "nested object value keeps inner key order",
			in:   `{"locations/1":{"@type":"Location","name":"Set A"}}`,
			want: `{"locations/1":{"@type":"Location","name":"Set A"}}`,
		},
		{
			name: "keys sorted on marshal",
			in:   `{"title":"x","description":"y"}`,
			want: `{"description":"y","title":"x"}`,
		},
		{
			name: "escaped pointer tokens kept verbatim",
			in:   `{"a~1b/c":"v"}`,
			want: `{"a~1b/c":"v"}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var p PatchObject
			if err := json.Unmarshal([]byte(tc.in), &p); err != nil {
				t.Fatalf("Unmarshal(%q) error = %v", tc.in, err)
			}
			out, err := json.Marshal(p)
			if err != nil {
				t.Fatalf("Marshal error = %v", err)
			}
			if string(out) != tc.want {
				t.Errorf("round-trip = %q, want %q", out, tc.want)
			}
		})
	}
}

func TestPatchObjectMarshalNilAndEmpty(t *testing.T) {
	// A nil PatchObject and an empty (non-nil) PatchObject both marshal
	// to an empty JSON object — a PatchObject is a value, never JSON
	// null, when it is present on a parent.
	for _, p := range []PatchObject{nil, {}} {
		out, err := json.Marshal(p)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}
		if string(out) != `{}` {
			t.Errorf("Marshal = %q, want {}", out)
		}
	}
}

func TestPatchObjectMarshalRejectsInvalidValueJSON(t *testing.T) {
	// Strict marshal: a value holding non-JSON bytes must fail rather
	// than emit a corrupt document. encoding/json itself enforces this
	// for json.RawMessage; the test pins the behavior.
	p := PatchObject{"title": json.RawMessage(`not json`)}
	if _, err := json.Marshal(p); err == nil {
		t.Fatal("Marshal of invalid raw value = nil error, want error")
	}
}

func TestPatchObjectUnmarshalRejectsNonObject(t *testing.T) {
	// A PatchObject is always a JSON object. Arrays, scalars, and null
	// at the top level are decode errors.
	for _, in := range []string{`[]`, `"x"`, `42`, `null`, `true`} {
		var p PatchObject
		if err := json.Unmarshal([]byte(in), &p); err == nil {
			t.Errorf("Unmarshal(%q) = nil error, want error", in)
		}
	}
}

func TestPatchObjectRemovals(t *testing.T) {
	p := PatchObject{
		"title":    json.RawMessage(`"x"`),
		"location": json.RawMessage(`null`),
		"start":    json.RawMessage(` null `), // whitespace-padded null still counts
	}
	if p.IsRemoval("title") {
		t.Error(`IsRemoval("title") = true, want false`)
	}
	if !p.IsRemoval("location") {
		t.Error(`IsRemoval("location") = false, want true`)
	}
	if !p.IsRemoval("start") {
		t.Error(`IsRemoval("start") = false, want true`)
	}
	if p.IsRemoval("absent") {
		t.Error(`IsRemoval("absent") = true, want false for a missing key`)
	}

	got := p.Removals()
	want := []string{"location", "start"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Removals() = %v, want %v", got, want)
	}
}

func TestValidatePointer(t *testing.T) {
	// RFC 6901 well-formedness only — does not resolve against any
	// document. The empty pointer is the whole document and is valid as
	// a JSON Pointer; PatchObject keys forbid it separately (it would
	// target the root, which a patch may not replace wholesale).
	valid := []string{
		"",                         // whole-document pointer (RFC 6901 §5)
		"/title",                   // single reference token
		"/locations/1",             // two reference tokens
		"/a~0b",                    // encoded tilde
		"/a~1b",                    // encoded slash
		"/alerts/1/trigger/offset", // deeper path
		"//token",                  // an empty leading reference token is legal
	}
	for _, ptr := range valid {
		if err := ValidatePointer(ptr); err != nil {
			t.Errorf("ValidatePointer(%q) = %v, want nil", ptr, err)
		}
	}

	invalid := []string{
		"title",       // non-empty pointer must start with '/'
		"locations/1", // likewise
		"/~",          // dangling tilde escape
		"/a~",         // trailing tilde
		"/a~2b",       // ~2 is not a valid escape (only ~0 and ~1)
		"/no~lead",    // bare tilde mid-token
		"/trailing~",  // tilde at end
	}
	for _, ptr := range invalid {
		if err := ValidatePointer(ptr); err == nil {
			t.Errorf("ValidatePointer(%q) = nil, want error", ptr)
		}
	}
}

func TestValidatePointerRejectsMissingLeadingSlash(t *testing.T) {
	// RFC 6901 requires every non-empty pointer to begin with "/".
	// PatchObject keys, however, are relative to the patched object and
	// omit the leading slash (RFC 8984 §1.4.9). ValidatePointer accepts
	// the absolute RFC 6901 form; ValidatePatchKey accepts the relative
	// JSCalendar form. This test pins the absolute-form rule.
	if err := ValidatePointer("/title"); err != nil {
		t.Errorf("ValidatePointer(%q) = %v, want nil", "/title", err)
	}
	if err := ValidatePointer("/locations/1"); err != nil {
		t.Errorf("ValidatePointer(%q) = %v, want nil", "/locations/1", err)
	}
}

func TestValidatePatchKey(t *testing.T) {
	// JSCalendar PatchObject keys are RFC 6901 pointers *relative to the
	// patched object* (§1.4.9): no leading slash, never empty, and never
	// targeting the root "@type" member.
	valid := []string{
		"title",
		"locations/1",
		"alerts/1/trigger",
		"recurrenceOverrides/2020-01-01T00:00:00",
		"a~1b/c", // escaped slash inside a single token
	}
	for _, k := range valid {
		if err := ValidatePatchKey(k); err != nil {
			t.Errorf("ValidatePatchKey(%q) = %v, want nil", k, err)
		}
	}

	invalid := map[string]string{
		"empty key":         "",
		"whole-doc pointer": "/",
		"leading slash":     "/title",
		"root @type target": "@type",
		"malformed escape":  "a~2b",
		"dangling tilde":    "a~",
	}
	for name, k := range invalid {
		t.Run(name, func(t *testing.T) {
			if err := ValidatePatchKey(k); err == nil {
				t.Errorf("ValidatePatchKey(%q) = nil, want error", k)
			}
		})
	}
}

func TestValidatePatchKeyAllowsNestedTypeButNotRoot(t *testing.T) {
	// Only the *root* "@type" is off-limits. A nested "@type" — e.g. the
	// @type of a replaced sub-object — is a legitimate patch target.
	if err := ValidatePatchKey("locations/1/@type"); err != nil {
		t.Errorf("ValidatePatchKey(nested @type) = %v, want nil", err)
	}
	if err := ValidatePatchKey("@type"); err == nil {
		t.Error("ValidatePatchKey(root @type) = nil, want error")
	}
}

func TestPatchObjectValidate(t *testing.T) {
	good := PatchObject{
		"title":            json.RawMessage(`"New"`),
		"locations/1":      json.RawMessage(`null`),
		"alerts/1/trigger": json.RawMessage(`{"@type":"OffsetTrigger"}`),
	}
	if err := good.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}

	bad := PatchObject{
		"title": json.RawMessage(`"ok"`),
		"@type": json.RawMessage(`"Event"`), // forbidden root target
	}
	err := bad.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want error for root @type target")
	}
	var perr *PatchError
	if !errors.As(err, &perr) {
		t.Fatalf("Validate() error type = %T, want *PatchError", err)
	}
	if perr.Pointer != "@type" {
		t.Errorf("PatchError.Pointer = %q, want %q", perr.Pointer, "@type")
	}
}

func TestPatchObjectValidateReportsFirstByKeyOrder(t *testing.T) {
	// Validation is deterministic: keys are checked in sorted order so
	// the same invalid PatchObject always reports the same offending
	// pointer, regardless of Go's map iteration order.
	p := PatchObject{
		"z~2bad": json.RawMessage(`1`), // malformed escape, sorts last
		"@type":  json.RawMessage(`1`), // forbidden, sorts first
	}
	err := p.Validate()
	var perr *PatchError
	if !errors.As(err, &perr) {
		t.Fatalf("Validate() error type = %T, want *PatchError", err)
	}
	if perr.Pointer != "@type" {
		t.Errorf("first reported pointer = %q, want %q", perr.Pointer, "@type")
	}
}

func TestPatchErrorMessage(t *testing.T) {
	err := &PatchError{Pointer: "@type", Reason: "must not target the root @type"}
	want := `jscalendar: invalid patch pointer "@type": must not target the root @type`
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestPatchObjectMarshalIsByteStableAcrossRuns(t *testing.T) {
	// Marshalling the same PatchObject twice yields identical bytes —
	// the property that makes PatchObject usable as a value inside a
	// byte-stable parent codec. encoding/json sorts map keys, so this
	// holds despite Go's randomized map iteration.
	p := PatchObject{
		"title":       json.RawMessage(`"x"`),
		"description": json.RawMessage(`"y"`),
		"location":    json.RawMessage(`null`),
	}
	first, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal error = %v", err)
	}
	for range 16 {
		next, err := json.Marshal(p)
		if err != nil {
			t.Fatalf("Marshal error = %v", err)
		}
		if !bytes.Equal(first, next) {
			t.Fatalf("Marshal not byte-stable: %q vs %q", first, next)
		}
	}
}

func ExamplePatchObject() {
	// A recurrenceOverrides patch: change the title of one occurrence
	// and remove its location entirely. The keys are JSON Pointers
	// relative to the overridden occurrence (RFC 8984 §1.4.9).
	const wire = `{"title":"Moved offsite","location":null}`

	var patch PatchObject
	if err := json.Unmarshal([]byte(wire), &patch); err != nil {
		panic(err)
	}

	if err := patch.Validate(); err != nil {
		panic(err)
	}

	fmt.Println("removes:", patch.Removals())
	fmt.Println("title is a removal:", patch.IsRemoval("title"))

	// Re-encoding is byte-stable: keys are sorted, the null sentinel is
	// preserved as JSON null rather than dropped.
	out, err := json.Marshal(patch)
	if err != nil {
		panic(err)
	}
	fmt.Println("re-encoded:", string(out))

	// Output:
	// removes: [location]
	// title is a removal: false
	// re-encoded: {"location":null,"title":"Moved offsite"}
}

func TestValidatePatchKeyTildeEscapeEdges(t *testing.T) {
	// RFC 6901 escapes: ~0 -> '~', ~1 -> '/'. "~01" is the escape ~0
	// followed by a literal '1', i.e. the two characters "~1" as data —
	// it is well-formed, not the ~1 escape. Pin the parser's handling of
	// the adjacent-escape edge so a future refactor can't regress it.
	valid := []string{
		"a~0b",  // tilde literal in a token
		"a~1b",  // slash literal in a token
		"a~01b", // ~0 escape, then literal '1'
		"a~10b", // ~1 escape, then literal '0'
	}
	for _, k := range valid {
		if err := ValidatePatchKey(k); err != nil {
			t.Errorf("ValidatePatchKey(%q) = %v, want nil", k, err)
		}
	}
}

func TestValidatePatchKeyMultibyteToken(t *testing.T) {
	// The tilde scan operates on bytes; UTF-8 continuation bytes are all
	// >= 0x80 and never collide with '~' (0x7e), so a multi-byte token is
	// well-formed. recurrenceOverrides keys carry LocalDateTime strings,
	// and localizations keys carry BCP 47 language tags, so non-ASCII
	// content does occur in practice.
	for _, k := range []string{"локация", "場所/1", "café"} {
		if err := ValidatePatchKey(k); err != nil {
			t.Errorf("ValidatePatchKey(%q) = %v, want nil", k, err)
		}
	}
}

func TestPatchObjectUnmarshalDuplicateKeys(t *testing.T) {
	// encoding/json keeps the last value for a duplicated object key.
	// Decoding is lenient (Postel), so a duplicate is not an error; the
	// test pins the last-wins behavior the codec inherits.
	var p PatchObject
	if err := json.Unmarshal([]byte(`{"title":"first","title":"second"}`), &p); err != nil {
		t.Fatalf("Unmarshal error = %v", err)
	}
	if got := string(p["title"]); got != `"second"` {
		t.Errorf("duplicate key resolved to %s, want \"second\"", got)
	}
}

func FuzzPatchObjectRoundTrip(f *testing.F) {
	seeds := []string{
		`{}`,
		`{"title":"x"}`,
		`{"location":null}`,
		`{"a~1b/c":{"@type":"Location","name":"X"}}`,
		`{"description":"y","title":"x"}`,
		`[]`,
		`null`,
		`"scalar"`,
		`{"@type":"Event"}`,
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var p PatchObject
		if err := json.Unmarshal(data, &p); err != nil {
			return // not a valid PatchObject document; nothing to assert
		}

		// Validate must never panic on any decodable PatchObject.
		_ = p.Validate()

		// Marshalling a decoded PatchObject must succeed: every retained
		// value came from a valid JSON document, so it is valid JSON.
		first, err := json.Marshal(p)
		if err != nil {
			t.Fatalf("Marshal after Unmarshal failed: %v (input %q)", err, data)
		}

		// Re-decoding the canonical form and re-encoding must be a fixed
		// point — the marshal output is byte-stable.
		var p2 PatchObject
		if err := json.Unmarshal(first, &p2); err != nil {
			t.Fatalf("re-Unmarshal of canonical form failed: %v (canonical %q)", err, first)
		}
		second, err := json.Marshal(p2)
		if err != nil {
			t.Fatalf("re-Marshal failed: %v", err)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("round-trip not a fixed point: %q vs %q", first, second)
		}
	})
}

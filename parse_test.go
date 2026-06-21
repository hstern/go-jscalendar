// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestParseDispatch covers the core routing contract: an object's "@type"
// member selects the concrete Go type Parse returns. The three recognized
// discriminators each map to a distinct pointer type, and the decoded value
// carries the spec-mandated fields through.
func TestParseDispatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		check func(t *testing.T, got any)
	}{
		{
			name:  "event",
			input: `{"@type":"Event","uid":"e1","start":"2020-01-15T13:00:00"}`,
			check: func(t *testing.T, got any) {
				ev, ok := got.(*Event)
				if !ok {
					t.Fatalf("got %T, want *Event", got)
				}
				if ev.UID != "e1" {
					t.Errorf("UID = %q, want %q", ev.UID, "e1")
				}
				if ev.Start == nil {
					t.Errorf("Start = nil, want decoded LocalDateTime")
				}
			},
		},
		{
			name:  "task",
			input: `{"@type":"Task","uid":"t1","title":"Do something"}`,
			check: func(t *testing.T, got any) {
				tk, ok := got.(*Task)
				if !ok {
					t.Fatalf("got %T, want *Task", got)
				}
				if tk.Title != "Do something" {
					t.Errorf("Title = %q, want %q", tk.Title, "Do something")
				}
			},
		},
		{
			name:  "group",
			input: `{"@type":"Group","uid":"g1","entries":[{"@type":"Event","uid":"e1"}]}`,
			check: func(t *testing.T, got any) {
				gr, ok := got.(*Group)
				if !ok {
					t.Fatalf("got %T, want *Group", got)
				}
				if gr.NumEntries() != 1 {
					t.Errorf("NumEntries = %d, want 1", gr.NumEntries())
				}
			},
		},
		{
			// Member order is irrelevant: "@type" need not lead for Parse to
			// route, matching the lenient decode of the per-type unmarshalers.
			name:  "type not first",
			input: `{"uid":"e1","title":"x","@type":"Event"}`,
			check: func(t *testing.T, got any) {
				if _, ok := got.(*Event); !ok {
					t.Fatalf("got %T, want *Event", got)
				}
			},
		},
		{
			// Leading insignificant whitespace must not defeat the object
			// guard or the "@type" peek.
			name:  "leading whitespace",
			input: "  \n\t{\"@type\":\"Task\",\"uid\":\"t1\"}",
			check: func(t *testing.T, got any) {
				if _, ok := got.(*Task); !ok {
					t.Fatalf("got %T, want *Task", got)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse([]byte(tc.input))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			tc.check(t, got)
		})
	}
}

// TestParseUnknownType verifies that an absent or unrecognized "@type" is a
// typed *UnknownTypeError rather than a silent default — the issue's central
// requirement. The Absent flag distinguishes "no @type" from "@type we don't
// handle", and the verbatim Type is surfaced for the unknown case.
func TestParseUnknownType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantType   string
		wantAbsent bool
	}{
		{"absent", `{"uid":"x"}`, "", true},
		{"null type", `{"@type":null,"uid":"x"}`, "", true},
		{"unknown", `{"@type":"Journal","uid":"x"}`, "Journal", false},
		{"empty string type", `{"@type":"","uid":"x"}`, "", false},
		{"lowercase event is unknown", `{"@type":"event"}`, "event", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := Parse([]byte(tc.input))
			if err == nil {
				t.Fatalf("Parse(%s) = nil error, want *UnknownTypeError", tc.input)
			}
			var ute *UnknownTypeError
			if !errors.As(err, &ute) {
				t.Fatalf("error %v (%T), want *UnknownTypeError", err, err)
			}
			if ute.Type != tc.wantType {
				t.Errorf("Type = %q, want %q", ute.Type, tc.wantType)
			}
			if ute.Absent != tc.wantAbsent {
				t.Errorf("Absent = %v, want %v", ute.Absent, tc.wantAbsent)
			}
		})
	}
}

// TestParseNonObject confirms a non-object top-level value is a structural
// error, distinct from the typed UnknownTypeError that an absent discriminator
// on an object produces. A bare scalar, array, or null never carries an
// "@type", so the peek cannot even begin.
func TestParseNonObject(t *testing.T) {
	t.Parallel()

	for _, input := range []string{`[]`, `"Event"`, `42`, `null`, `  `, ``} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			_, err := Parse([]byte(input))
			if err == nil {
				t.Fatalf("Parse(%q) = nil error, want structural error", input)
			}
			var ute *UnknownTypeError
			if errors.As(err, &ute) {
				t.Errorf("Parse(%q) = %T, want a plain structural error", input, err)
			}
		})
	}
}

// TestParseNonStringType confirms that an "@type" present but not a JSON string
// (a number, object, array, or bool) is a structural decode error rather than a
// typed UnknownTypeError — the discriminator is defined as a string (RFC 8984,
// Section 1.4.1), so a non-string value is malformed input, not an unknown type.
func TestParseNonStringType(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		`{"@type":123}`,
		`{"@type":true}`,
		`{"@type":["Event"]}`,
		`{"@type":{"x":1}}`,
	} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			_, err := Parse([]byte(input))
			if err == nil {
				t.Fatalf("Parse(%s) = nil error, want decode error", input)
			}
			var ute *UnknownTypeError
			if errors.As(err, &ute) {
				t.Errorf("Parse(%s) = %T, want a structural decode error", input, err)
			}
		})
	}
}

// TestParseOrderIndependence pins that a real Section 6 figure re-marshals to
// the same canonical bytes regardless of the input member order — the property
// that makes the byte-stable corpus meaningful (the codec, not the input
// layout, fixes the output order). It shuffles the members of the simple-event
// figure and asserts the round trip lands on the canonical form.
func TestParseOrderIndependence(t *testing.T) {
	t.Parallel()

	canonical, err := os.ReadFile(filepath.Join(rfc8984CorpusDir, "6.1-simple-event.json"))
	if err != nil {
		t.Fatalf("read canonical: %v", err)
	}
	want := trimTrailingNewline(string(canonical))

	// The same members as 6.1, deliberately reordered and with "@type" not
	// first, as a producer with different field order might emit.
	reordered := `{"duration":"PT1H","timeZone":"America/New_York",` +
		`"title":"Some event","start":"2020-01-15T13:00:00",` +
		`"updated":"2020-01-02T18:23:04Z",` +
		`"uid":"a8df6573-0474-496d-8496-033ad45d7fea","@type":"Event"}`

	obj, err := Parse([]byte(reordered))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(got) != want {
		t.Errorf("reordered input did not canonicalize\n got: %s\nwant: %s", got, want)
	}
}

// TestUnknownTypeErrorMessage pins the two distinct human-readable forms so the
// Absent and unknown-value cases read differently in logs.
func TestUnknownTypeErrorMessage(t *testing.T) {
	t.Parallel()

	absent := (&UnknownTypeError{Absent: true}).Error()
	if want := `jscalendar: cannot parse object: no "@type" member`; absent != want {
		t.Errorf("absent message = %q, want %q", absent, want)
	}
	unknown := (&UnknownTypeError{Type: "Journal"}).Error()
	if want := `jscalendar: cannot parse object: unknown "@type" "Journal"`; unknown != want {
		t.Errorf("unknown message = %q, want %q", unknown, want)
	}
}

// TestGroupEntryDispatch checks that a parsed Group's lazily retained entries
// dispatch on their own "@type" through Group.Entry, the same routing Parse
// applies to a top-level object. The accessor is the bridge from the raw
// json.RawMessage entries to typed Event/Task values.
func TestGroupEntryDispatch(t *testing.T) {
	t.Parallel()

	const input = `{"@type":"Group","uid":"g1","entries":[` +
		`{"@type":"Event","uid":"e1","title":"Some event"},` +
		`{"@type":"Task","uid":"t1","title":"Do something"}]}`

	obj, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	g, ok := obj.(*Group)
	if !ok {
		t.Fatalf("got %T, want *Group", obj)
	}
	if g.NumEntries() != 2 {
		t.Fatalf("NumEntries = %d, want 2", g.NumEntries())
	}

	e0, err := g.Entry(0)
	if err != nil {
		t.Fatalf("Entry(0): %v", err)
	}
	ev, ok := e0.(*Event)
	if !ok {
		t.Fatalf("Entry(0) = %T, want *Event", e0)
	}
	if ev.UID != "e1" {
		t.Errorf("Entry(0).UID = %q, want %q", ev.UID, "e1")
	}

	e1, err := g.Entry(1)
	if err != nil {
		t.Fatalf("Entry(1): %v", err)
	}
	tk, ok := e1.(*Task)
	if !ok {
		t.Fatalf("Entry(1) = %T, want *Task", e1)
	}
	if tk.Title != "Do something" {
		t.Errorf("Entry(1).Title = %q, want %q", tk.Title, "Do something")
	}
}

// TestGroupEntryUnknownType verifies that an entry with an absent or
// unrecognized "@type" — including a nested Group, which Entry does not
// construct because entries are constrained to Event and Task (RFC 8984,
// Section 5.3.1) — surfaces a *UnknownTypeError rather than decoding silently.
func TestGroupEntryUnknownType(t *testing.T) {
	t.Parallel()

	const input = `{"@type":"Group","uid":"g1","entries":[` +
		`{"uid":"x"},{"@type":"Group","uid":"nested"}]}`

	obj, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	g := obj.(*Group)

	if _, err := g.Entry(0); err == nil {
		t.Errorf("Entry(0) = nil error, want UnknownTypeError for absent @type")
	} else {
		var ute *UnknownTypeError
		if !errors.As(err, &ute) || !ute.Absent {
			t.Errorf("Entry(0) error = %v, want absent UnknownTypeError", err)
		}
	}

	if _, err := g.Entry(1); err == nil {
		t.Errorf("Entry(1) = nil error, want UnknownTypeError for nested Group")
	} else {
		var ute *UnknownTypeError
		if !errors.As(err, &ute) || ute.Type != "Group" {
			t.Errorf("Entry(1) error = %v, want UnknownTypeError Type=Group", err)
		}
	}
}

// rfc8984CorpusDir holds the canonical RFC 8984 Section 6 example figures, each
// stored in the library's own marshaled form (see the test below for what
// "canonical" and "byte-stable" mean for this corpus).
const rfc8984CorpusDir = "testdata/rfc8984"

// TestRFC8984Section6RoundTrip is the phase-3 acceptance gate: every RFC 8984
// Section 6 example figure round-trips byte-stably through the codec.
//
// What "byte-stable" means for this corpus. The figures as printed in the RFC
// are pretty-printed with members in human-readable, not canonical, order — so
// a literal byte comparison against the RFC text would test whitespace and the
// editors' member ordering, not the codec. Instead, each corpus file is stored
// in the library's canonical marshaled form: compact (no insignificant
// whitespace), "@type" first, known properties in struct-declaration order, and
// unknown ("Extra") members in sorted key order. Byte-stability is then the
// exact, byte-for-byte assertion that
//
//	Marshal(Parse(canonical)) == canonical
//
// holds for every figure. This is the property that matters for interop: a
// JSCalendar object decoded and re-encoded by this library reproduces its bytes
// exactly, so a consumer that pins exact bytes (a signature, a cache key, a
// golden test) is never perturbed by a no-op round trip. Members the core model
// does not yet type — locations, participants, localizations, and the like, due
// in a later phase — survive verbatim through the Extra open-extension seam,
// which is precisely what makes the round trip lossless for the full corpus
// rather than only the simple figures.
//
// The corpus is regenerated from the hand-transcribed source figures by the
// generator that seeded testdata/rfc8984; if a codec change alters the
// canonical form, this test fails and the corpus must be regenerated and
// reviewed, which is the intended tripwire.
func TestRFC8984Section6RoundTrip(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(filepath.Join(rfc8984CorpusDir, "*.json"))
	if err != nil {
		t.Fatalf("glob corpus: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no corpus files under %s", rfc8984CorpusDir)
	}
	// All ten Section 6 figures must be present; a dropped file would silently
	// shrink the acceptance surface.
	if len(files) != 10 {
		t.Errorf("found %d corpus files, want 10 (RFC 8984 Section 6 has 10 figures)", len(files))
	}

	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			t.Parallel()

			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			// The file is stored canonical-plus-trailing-newline for editor
			// friendliness; the canonical bytes are the content without it.
			canonical := []byte(trimTrailingNewline(string(raw)))

			obj, err := Parse(canonical)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			got, err := json.Marshal(obj)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			if string(got) != string(canonical) {
				t.Errorf("round trip not byte-stable\n got: %s\nwant: %s", got, canonical)
			}

			// A second round trip must reproduce the same bytes — the codec is
			// idempotent, not merely stable on the first pass.
			obj2, err := Parse(got)
			if err != nil {
				t.Fatalf("re-Parse: %v", err)
			}
			got2, err := json.Marshal(obj2)
			if err != nil {
				t.Fatalf("re-Marshal: %v", err)
			}
			if string(got2) != string(got) {
				t.Errorf("second round trip diverged\nfirst:  %s\nsecond: %s", got, got2)
			}
		})
	}
}

// TestRFC8984Section6Dispatch confirms each corpus figure routes to the
// concrete type its "@type" names — the dispatch contract exercised against the
// real spec examples rather than synthetic inputs.
func TestRFC8984Section6Dispatch(t *testing.T) {
	t.Parallel()

	wantType := map[string]string{
		"6.1-simple-event.json":                       "*jscalendar.Event",
		"6.2-simple-task.json":                        "*jscalendar.Task",
		"6.3-simple-group.json":                       "*jscalendar.Group",
		"6.4-all-day-event.json":                      "*jscalendar.Event",
		"6.5-task-with-due-date.json":                 "*jscalendar.Task",
		"6.6-event-with-end-time-zone.json":           "*jscalendar.Event",
		"6.7-floating-time-event.json":                "*jscalendar.Event",
		"6.8-multiple-locations-localization.json":    "*jscalendar.Event",
		"6.9-recurring-event-with-overrides.json":     "*jscalendar.Event",
		"6.10-recurring-event-with-participants.json": "*jscalendar.Event",
	}

	files, err := filepath.Glob(filepath.Join(rfc8984CorpusDir, "*.json"))
	if err != nil {
		t.Fatalf("glob corpus: %v", err)
	}
	for _, file := range files {
		base := filepath.Base(file)
		t.Run(base, func(t *testing.T) {
			t.Parallel()
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			obj, err := Parse(raw)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if got := typeName(obj); got != wantType[base] {
				t.Errorf("Parse(%s) = %s, want %s", base, got, wantType[base])
			}
		})
	}
}

// trimTrailingNewline strips a single trailing '\n' if present, recovering the
// canonical bytes from a corpus file stored with an editor-friendly newline.
func trimTrailingNewline(s string) string {
	if n := len(s); n > 0 && s[n-1] == '\n' {
		return s[:n-1]
	}
	return s
}

// typeName returns a stable string identity for the three concrete top-level
// types so a test can assert on dispatch without importing reflect.
func typeName(v any) string {
	switch v.(type) {
	case *Event:
		return "*jscalendar.Event"
	case *Task:
		return "*jscalendar.Task"
	case *Group:
		return "*jscalendar.Group"
	default:
		return "unknown"
	}
}

// ExampleParse shows routing an object to its concrete type on the "@type"
// discriminator, then recovering the type with a type switch.
func ExampleParse() {
	obj, err := Parse([]byte(`{"@type":"Event","uid":"e1","title":"Some event"}`))
	if err != nil {
		panic(err)
	}
	switch v := obj.(type) {
	case *Event:
		fmt.Printf("Event %s: %s\n", v.UID, v.Title)
	case *Task:
		fmt.Printf("Task %s: %s\n", v.UID, v.Title)
	case *Group:
		fmt.Printf("Group %s with %d entries\n", v.UID, v.NumEntries())
	}
	// Output: Event e1: Some event
}

// ExampleParse_unknownType shows that an absent or unrecognized "@type" yields a
// typed *UnknownTypeError rather than a silently defaulted object.
func ExampleParse_unknownType() {
	_, err := Parse([]byte(`{"@type":"Journal","uid":"x"}`))
	var ute *UnknownTypeError
	if errors.As(err, &ute) {
		fmt.Printf("unknown type %q (absent=%v)\n", ute.Type, ute.Absent)
	}
	// Output: unknown type "Journal" (absent=false)
}

// ExampleGroup_Entry shows iterating a parsed Group's entries, each dispatched
// to its own concrete type by Entry.
func ExampleGroup_Entry() {
	obj, err := Parse([]byte(`{"@type":"Group","uid":"g1","entries":[` +
		`{"@type":"Event","uid":"e1","title":"Some event"},` +
		`{"@type":"Task","uid":"t1","title":"Do something"}]}`))
	if err != nil {
		panic(err)
	}
	g := obj.(*Group)
	for i := range g.NumEntries() {
		entry, err := g.Entry(i)
		if err != nil {
			panic(err)
		}
		switch v := entry.(type) {
		case *Event:
			fmt.Printf("event: %s\n", v.Title)
		case *Task:
			fmt.Printf("task: %s\n", v.Title)
		}
	}
	// Output:
	// event: Some event
	// task: Do something
}

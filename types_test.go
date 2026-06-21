// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"bytes"
	"encoding/json"
	"testing"
)

// ptrUint returns a pointer to v, for the pointer-typed optional fields.
func ptrUint(v uint) *uint { return &v }

// reencode round-trips raw JSON through v: it unmarshals raw into v, then
// marshals v back out. The returned bytes are what a decode-then-encode of
// the input produces, which the round-trip tests compare against the
// canonical input.
func reencode[T any](t *testing.T, raw string) []byte {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("unmarshal into %T: %v", v, err)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}
	return out
}

// TestTopLevelTypeEmittedFirst checks that the mandatory "@type" member
// (RFC 8984, Section 1.4.1) is the first key in the marshaled object for
// each top-level type. The default encoding satisfies this only because
// Type is the first declared field, so the test guards against a future
// field reorder silently breaking the "emit @type first" rule before the
// dedicated codec lands.
func TestTopLevelTypeEmittedFirst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		obj  any
		want string
	}{
		{"event", &Event{Type: "Event", UID: "e1"}, `{"@type":"Event"`},
		{"task", &Task{Type: "Task", UID: "t1"}, `{"@type":"Task"`},
		{"group", &Group{Type: "Group", UID: "g1"}, `{"@type":"Group"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out, err := json.Marshal(tc.obj)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if !bytes.HasPrefix(out, []byte(tc.want)) {
				t.Errorf("marshal = %s, want prefix %s", out, tc.want)
			}
		})
	}
}

// TestEventRoundTrip checks that a fully-populated Event decodes and
// re-encodes byte-for-byte. The input is written in canonical key order
// (declaration order of the struct, which encoding/json preserves) so the
// comparison is exact.
func TestEventRoundTrip(t *testing.T) {
	t.Parallel()

	const canonical = `{` +
		`"@type":"Event",` +
		`"uid":"event-uid-1",` +
		`"prodId":"//Example//EN",` +
		`"created":"2020-01-01T08:00:00Z",` +
		`"updated":"2020-01-02T09:30:00Z",` +
		`"sequence":3,` +
		`"method":"request",` +
		`"title":"Team sync",` +
		`"description":"Weekly team sync meeting.",` +
		`"descriptionContentType":"text/plain",` +
		`"showWithoutTime":true,` +
		`"keywords":{"meeting":true},` +
		`"categories":{"http://example.com/cat/work":true},` +
		`"color":"#aabbcc",` +
		`"recurrenceId":"2020-01-15T13:00:00",` +
		`"recurrenceIdTimeZone":"America/New_York",` +
		`"recurrenceRules":[{"@type":"RecurrenceRule","frequency":"weekly"}],` +
		`"excludedRecurrenceRules":[{"@type":"RecurrenceRule","frequency":"daily"}],` +
		`"recurrenceOverrides":{"2020-01-22T13:00:00":{"title":"Moved sync"}},` +
		`"priority":5,` +
		`"freeBusyStatus":"busy",` +
		`"privacy":"private",` +
		`"useDefaultAlerts":true,` +
		`"start":"2020-01-15T13:00:00",` +
		`"duration":"PT1H",` +
		`"status":"confirmed",` +
		`"timeZone":"America/New_York"` +
		`}`

	got := reencode[Event](t, canonical)
	if string(got) != canonical {
		t.Errorf("round-trip mismatch:\n got: %s\nwant: %s", got, canonical)
	}
}

// TestTaskRoundTrip checks a fully-populated Task, including the Task-only
// scheduling members (due, estimatedDuration, percentComplete, progress).
func TestTaskRoundTrip(t *testing.T) {
	t.Parallel()

	const canonical = `{` +
		`"@type":"Task",` +
		`"uid":"task-uid-1",` +
		`"prodId":"//Example//EN",` +
		`"created":"2020-01-01T08:00:00Z",` +
		`"updated":"2020-01-02T09:30:00Z",` +
		`"sequence":1,` +
		`"method":"request",` +
		`"title":"Write report",` +
		`"description":"Draft the quarterly report.",` +
		`"descriptionContentType":"text/plain",` +
		`"showWithoutTime":true,` +
		`"keywords":{"report":true},` +
		`"categories":{"http://example.com/cat/work":true},` +
		`"color":"#112233",` +
		`"recurrenceId":"2020-02-01T09:00:00",` +
		`"recurrenceIdTimeZone":"Etc/UTC",` +
		`"recurrenceRules":[{"@type":"RecurrenceRule","frequency":"monthly"}],` +
		`"excludedRecurrenceRules":[{"@type":"RecurrenceRule","frequency":"weekly"}],` +
		`"recurrenceOverrides":{"2020-03-01T09:00:00":{"percentComplete":50}},` +
		`"priority":7,` +
		`"freeBusyStatus":"free",` +
		`"privacy":"public",` +
		`"useDefaultAlerts":true,` +
		`"due":"2020-02-10T17:00:00",` +
		`"start":"2020-02-01T09:00:00",` +
		`"estimatedDuration":"PT4H",` +
		`"percentComplete":25,` +
		`"progress":"in-process",` +
		`"timeZone":"Etc/UTC"` +
		`}`

	got := reencode[Task](t, canonical)
	if string(got) != canonical {
		t.Errorf("round-trip mismatch:\n got: %s\nwant: %s", got, canonical)
	}
}

// TestGroupRoundTrip checks a Group, including that Entries members are
// retained verbatim as raw JSON (key order and whitespace inside a member
// are preserved because the member is json.RawMessage, not a decoded
// struct).
func TestGroupRoundTrip(t *testing.T) {
	t.Parallel()

	const canonical = `{` +
		`"@type":"Group",` +
		`"uid":"group-uid-1",` +
		`"prodId":"//Example//EN",` +
		`"created":"2020-01-01T08:00:00Z",` +
		`"updated":"2020-01-02T09:30:00Z",` +
		`"sequence":2,` +
		`"method":"publish",` +
		`"title":"Project calendar",` +
		`"description":"All project events and tasks.",` +
		`"descriptionContentType":"text/plain",` +
		`"keywords":{"project":true},` +
		`"categories":{"http://example.com/cat/proj":true},` +
		`"color":"#445566",` +
		`"entries":[{"@type":"Event","uid":"e1"},{"@type":"Task","uid":"t1"}],` +
		`"source":"https://example.com/cal.ics"` +
		`}`

	got := reencode[Group](t, canonical)
	if string(got) != canonical {
		t.Errorf("round-trip mismatch:\n got: %s\nwant: %s", got, canonical)
	}
}

// TestMinimalRoundTrip checks that an object carrying only the two required
// members marshals to exactly those members — every optional field's
// omitempty tag suppresses it.
func TestMinimalRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		kind string
	}{
		{"event", `{"@type":"Event","uid":"e1"}`, "event"},
		{"task", `{"@type":"Task","uid":"t1"}`, "task"},
		{"group", `{"@type":"Group","uid":"g1"}`, "group"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got []byte
			switch tc.kind {
			case "event":
				got = reencode[Event](t, tc.raw)
			case "task":
				got = reencode[Task](t, tc.raw)
			case "group":
				got = reencode[Group](t, tc.raw)
			}
			if string(got) != tc.raw {
				t.Errorf("minimal round-trip = %s, want %s", got, tc.raw)
			}
		})
	}
}

// TestPercentCompleteZeroVsAbsent checks that PercentComplete distinguishes
// an explicit 0% (a meaningful value) from an absent property, which a
// plain uint field would conflate. An explicit 0 must survive the
// round-trip; an absent value must stay absent.
func TestPercentCompleteZeroVsAbsent(t *testing.T) {
	t.Parallel()

	t.Run("explicit zero is preserved", func(t *testing.T) {
		t.Parallel()
		const raw = `{"@type":"Task","uid":"t1","percentComplete":0}`
		got := reencode[Task](t, raw)
		if string(got) != raw {
			t.Errorf("round-trip = %s, want %s", got, raw)
		}
	})

	t.Run("absent stays absent", func(t *testing.T) {
		t.Parallel()
		var task Task
		if err := json.Unmarshal([]byte(`{"@type":"Task","uid":"t1"}`), &task); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if task.PercentComplete != nil {
			t.Errorf("PercentComplete = %v, want nil for an absent member", *task.PercentComplete)
		}
	})

	t.Run("set value is reachable", func(t *testing.T) {
		t.Parallel()
		task := Task{Type: "Task", UID: "t1", PercentComplete: ptrUint(100)}
		out, err := json.Marshal(task)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		const want = `{"@type":"Task","uid":"t1","percentComplete":100}`
		if string(out) != want {
			t.Errorf("marshal = %s, want %s", out, want)
		}
	})
}

// TestRecurrenceOverridesPatchKeysSorted checks that a recurrenceOverrides
// value is decoded as a [PatchObject], whose MarshalJSON emits its
// JSON-Pointer keys in sorted order for byte-stable output. The input here
// lists the patch keys out of sorted order ("title" before "priority"); the
// canonical re-encoding sorts them ("priority" before "title"), confirming
// the override value is a PatchObject and not an opaque blob.
func TestRecurrenceOverridesPatchKeysSorted(t *testing.T) {
	t.Parallel()

	const raw = `{"@type":"Event","uid":"e1",` +
		`"recurrenceOverrides":{"2020-01-22T13:00:00":{"title":"Changed","priority":9}}}`
	const want = `{"@type":"Event","uid":"e1",` +
		`"recurrenceOverrides":{"2020-01-22T13:00:00":{"priority":9,"title":"Changed"}}}`

	got := reencode[Event](t, raw)
	if string(got) != want {
		t.Errorf("round-trip = %s, want %s", got, want)
	}
}

// TestEntriesMemberOrderPreserved checks that a Group entry's internal key
// order survives the round-trip verbatim. Entries is []json.RawMessage
// precisely so a member's exact bytes are retained rather than re-encoded
// through a struct (which would re-sort or reorder keys). The member here
// lists "uid" before "@type" — a non-canonical order that a typed decode
// would normalize; as raw bytes it must come back unchanged.
func TestEntriesMemberOrderPreserved(t *testing.T) {
	t.Parallel()

	const raw = `{"@type":"Group","uid":"g1",` +
		`"entries":[{"uid":"e1","@type":"Event","title":"Kept order"}]}`

	got := reencode[Group](t, raw)
	if string(got) != raw {
		t.Errorf("round-trip = %s, want %s", got, raw)
	}
}

// TestMarshalFromConstructedValue checks marshaling a value built in Go
// (rather than decoded from JSON), exercising the encode path independently
// of decode. It guards against a struct-tag or field-order regression that a
// decode-then-encode test could mask if both directions shared the same bug.
func TestMarshalFromConstructedValue(t *testing.T) {
	t.Parallel()

	start, err := ParseLocalDateTime("2020-01-15T13:00:00")
	if err != nil {
		t.Fatalf("parse start: %v", err)
	}
	dur, err := ParseDuration("PT1H")
	if err != nil {
		t.Fatalf("parse duration: %v", err)
	}

	ev := Event{
		Type:     "Event",
		UID:      "e1",
		Title:    "Constructed",
		Start:    &start,
		Duration: &dur,
		TimeZone: "America/New_York",
	}

	out, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	const want = `{"@type":"Event","uid":"e1","title":"Constructed",` +
		`"start":"2020-01-15T13:00:00","duration":"PT1H","timeZone":"America/New_York"}`
	if string(out) != want {
		t.Errorf("marshal = %s, want %s", out, want)
	}
}

// TestNullPointerFieldDecodesToNil checks the lenient-unmarshal posture for
// an explicit JSON null on a pointer-typed optional field: it decodes to a
// nil pointer without error, the same as an absent member, rather than
// rejecting the input.
func TestNullPointerFieldDecodesToNil(t *testing.T) {
	t.Parallel()

	const raw = `{"@type":"Event","uid":"e1","duration":null,"start":null}`
	var ev Event
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		t.Fatalf("unmarshal explicit null: %v", err)
	}
	if ev.Duration != nil {
		t.Errorf("Duration = %v, want nil for explicit null", ev.Duration)
	}
	if ev.Start != nil {
		t.Errorf("Start = %v, want nil for explicit null", ev.Start)
	}
}

// TestUnknownMembersToleratedOnDecode checks the lenient-unmarshal posture:
// an unrecognized member decodes without error (it is dropped for now;
// lossless preservation via an Extra field arrives in a later phase). This
// pins the "decode never rejects for unknown members" behavior so a future
// change that adds Extra does not have to also relax a strict decoder.
func TestUnknownMembersToleratedOnDecode(t *testing.T) {
	t.Parallel()

	const raw = `{"@type":"Event","uid":"e1","x-vendor-thing":42,"futureSpecProp":"v"}`
	var ev Event
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		t.Fatalf("unmarshal with unknown members: %v", err)
	}
	if ev.UID != "e1" {
		t.Errorf("UID = %q, want %q", ev.UID, "e1")
	}
}

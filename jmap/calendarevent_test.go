// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jmap_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/hstern/go-jscalendar"
	"github.com/hstern/go-jscalendar/jmap"
)

// sampleCalendarEvent is the canonical wire form used across the round-trip
// tests: a JSCalendar Event ("@type" first, with start/duration/timeZone and an
// unknown vendor member) carrying the JMAP additions calendarIds, isDraft, and
// utcStart. The member order matches the codec's deterministic output —
// "@type", the Event's known members in declaration order, the Event's Extra in
// sorted key order, then the JMAP members in declaration order — so a decode
// followed by an encode must reproduce these exact bytes.
const sampleCalendarEvent = `{` +
	`"@type":"Event",` +
	`"uid":"a1b2c3","title":"Sprint review",` +
	`"start":"2026-06-21T14:00:00","duration":"PT1H","timeZone":"America/Toronto",` +
	`"example.com:flag":true,` +
	`"id":"E-7","calendarIds":{"work":true},"isDraft":true,"utcStart":"2026-06-21T18:00:00Z"` +
	`}`

func TestCalendarEventRoundTripByteStable(t *testing.T) {
	t.Parallel()

	var ce jmap.CalendarEvent
	if err := json.Unmarshal([]byte(sampleCalendarEvent), &ce); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// The JMAP members decoded onto the CalendarEvent's own fields.
	if ce.ID != "E-7" {
		t.Errorf("ID = %q, want %q", ce.ID, "E-7")
	}
	if got := ce.CalendarIDs["work"]; !got {
		t.Errorf("CalendarIDs[work] = %v, want true", got)
	}
	if !ce.IsDraft {
		t.Error("IsDraft = false, want true")
	}
	if ce.UTCStart == nil || ce.UTCStart.String() != "2026-06-21T18:00:00Z" {
		t.Errorf("UTCStart = %v, want 2026-06-21T18:00:00Z", ce.UTCStart)
	}

	// The JSCalendar members decoded onto the embedded Event.
	if ce.Event == nil {
		t.Fatal("embedded Event is nil")
	}
	if ce.UID != "a1b2c3" {
		t.Errorf("Event.UID = %q, want %q", ce.UID, "a1b2c3")
	}
	if ce.Title != "Sprint review" {
		t.Errorf("Event.Title = %q, want %q", ce.Title, "Sprint review")
	}

	// The unknown vendor member round-trips through the embedded Event's Extra.
	raw, ok := ce.Extra["example.com:flag"]
	if !ok {
		t.Fatalf("unknown member not captured in Event.Extra; Extra=%v", ce.Extra)
	}
	var flag bool
	if err := jscalendar.DecodeJSON(raw, &flag); err != nil {
		t.Fatalf("decode extra: %v", err)
	}
	if !flag {
		t.Error("example.com:flag = false, want true")
	}

	// Re-encode and require byte-identical output.
	out, err := json.Marshal(ce)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Equal(out, []byte(sampleCalendarEvent)) {
		t.Errorf("round trip not byte-stable\n got: %s\nwant: %s", out, sampleCalendarEvent)
	}
}

func TestCalendarEventTypeFirstAndNoJMAPWhenZero(t *testing.T) {
	t.Parallel()

	// A CalendarEvent with no JMAP members set marshals as a bare Event: the
	// "@type" discriminator leads and no empty JMAP members appear.
	ce := jmap.FromEvent(&jscalendar.Event{UID: "x"})
	out, err := json.Marshal(ce)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"@type":"Event","uid":"x"}`
	if string(out) != want {
		t.Errorf("marshal = %s, want %s", out, want)
	}
}

func TestCalendarEventNilEventMarshals(t *testing.T) {
	t.Parallel()

	// A CalendarEvent with a nil embedded Event still marshals as a valid
	// object: "@type":"Event" plus whatever JMAP members are set.
	ce := jmap.CalendarEvent{IsDraft: true, CalendarIDs: map[jscalendar.Id]bool{"c": true}}
	out, err := json.Marshal(ce)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"@type":"Event","calendarIds":{"c":true},"isDraft":true}`
	if string(out) != want {
		t.Errorf("marshal = %s, want %s", out, want)
	}

	// And it decodes back symmetrically.
	var got jmap.CalendarEvent
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !got.IsDraft || !got.CalendarIDs["c"] {
		t.Errorf("decoded JMAP members lost: %+v", got)
	}
}

func TestFromEventToEventRoundTrip(t *testing.T) {
	t.Parallel()

	ev := &jscalendar.Event{
		UID:      "evt-1",
		Title:    "Standup",
		Start:    mustLocal(t, "2026-06-21T09:00:00"),
		Duration: mustDuration(t, "PT15M"),
		TimeZone: "America/Toronto",
	}

	ce := jmap.FromEvent(ev)
	// FromEvent aliases the Event, it does not copy it.
	if ce.ToEvent() != ev {
		t.Error("ToEvent did not return the wrapped Event pointer")
	}

	// The shared JSCalendar properties survive the wrap unchanged.
	got := ce.ToEvent()
	if got.UID != ev.UID || got.Title != ev.Title || got.TimeZone != ev.TimeZone {
		t.Errorf("shared fields changed: got %+v want %+v", got, ev)
	}

	// JMAP fields default empty; calendarIds in particular is nil for the
	// caller to fill before a CalendarEvent/set create.
	if ce.CalendarIDs != nil {
		t.Errorf("CalendarIDs = %v, want nil", ce.CalendarIDs)
	}
}

func TestToEventNil(t *testing.T) {
	t.Parallel()

	var ce *jmap.CalendarEvent
	if ce.ToEvent() != nil {
		t.Error("nil CalendarEvent.ToEvent() should be nil")
	}

	empty := &jmap.CalendarEvent{}
	if empty.ToEvent() != nil {
		t.Error("CalendarEvent with nil Event should ToEvent() to nil")
	}
}

func TestConvertedEventValidates(t *testing.T) {
	t.Parallel()

	// An event decoded as a CalendarEvent, then unwrapped, must Validate
	// cleanly: the JMAP additions do not leak into the JSCalendar object and
	// break its conformance.
	var ce jmap.CalendarEvent
	if err := json.Unmarshal([]byte(sampleCalendarEvent), &ce); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := ce.ToEvent().Validate(); err != nil {
		t.Fatalf("unwrapped event failed validation: %v", err)
	}
}

func TestRestrictedReportsMethod(t *testing.T) {
	t.Parallel()

	clean := jmap.FromEvent(&jscalendar.Event{UID: "u"})
	if got := clean.Restricted(); len(got) != 0 {
		t.Errorf("Restricted() = %v, want empty", got)
	}

	withMethod := jmap.FromEvent(&jscalendar.Event{UID: "u", Method: "request"})
	got := withMethod.Restricted()
	if len(got) != 1 || got[0] != "method" {
		t.Errorf("Restricted() = %v, want [method]", got)
	}

	// The forbidden property is reported, not silently dropped: it still
	// round-trips on the wire.
	out, err := json.Marshal(withMethod)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Contains(out, []byte(`"method":"request"`)) {
		t.Errorf("method was dropped from the wire: %s", out)
	}
}

func TestUnmarshalRejectsNonObject(t *testing.T) {
	t.Parallel()

	for _, in := range []string{`[]`, `"x"`, `42`, `null`} {
		var ce jmap.CalendarEvent
		if err := json.Unmarshal([]byte(in), &ce); err == nil {
			t.Errorf("unmarshal(%s) succeeded, want error", in)
		}
	}
}

func TestUnmarshalToleratesMemberOrder(t *testing.T) {
	t.Parallel()

	// JMAP members before "@type", scrambled order: the codec must still route
	// each member correctly and produce the canonical byte-stable form.
	scrambled := `{"isDraft":true,"calendarIds":{"work":true},"id":"E-7",` +
		`"utcStart":"2026-06-21T18:00:00Z","example.com:flag":true,` +
		`"duration":"PT1H","timeZone":"America/Toronto","start":"2026-06-21T14:00:00",` +
		`"title":"Sprint review","uid":"a1b2c3","@type":"Event"}`

	var ce jmap.CalendarEvent
	if err := json.Unmarshal([]byte(scrambled), &ce); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := json.Marshal(ce)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Equal(out, []byte(sampleCalendarEvent)) {
		t.Errorf("scrambled input not normalized\n got: %s\nwant: %s", out, sampleCalendarEvent)
	}
}

func mustLocal(t *testing.T, s string) *jscalendar.LocalDateTime {
	t.Helper()
	v, err := jscalendar.ParseLocalDateTime(s)
	if err != nil {
		t.Fatalf("ParseLocalDateTime(%q): %v", s, err)
	}
	return &v
}

func mustDuration(t *testing.T, s string) *jscalendar.Duration {
	t.Helper()
	v, err := jscalendar.ParseDuration(s)
	if err != nil {
		t.Fatalf("ParseDuration(%q): %v", s, err)
	}
	return &v
}

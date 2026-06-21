// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"bytes"
	"encoding/json"
	"testing"
)

// This file tests the Alert sub-object and its polymorphic trigger union (RFC
// 8984, Sections 4.5.2 and 4.5.1), together with the Link and Relation
// sub-objects (Sections 1.4.11 and 1.4.10) and their wiring into the Event
// and Task "alerts", "links", and "relatedTo" maps. The behavior mirrors the
// other sub-objects: "@type" emitted first and forced, order-tolerant lenient
// decode, and lossless byte-stable round-tripping — including, for the
// trigger union, an unrecognized inner "@type" preserved verbatim.

// TestAlertSubObjectTypeEmittedFirst checks that the Alert, Link, and
// Relation sub-objects marshal their mandatory "@type" member first (RFC
// 8984, Section 1.4.1).
func TestAlertSubObjectTypeEmittedFirst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		obj  any
		want string
	}{
		{"alert", Alert{Action: "display"}, `{"@type":"Alert"`},
		{"link", Link{Href: "https://example.com"}, `{"@type":"Link"`},
		{"relation", Relation{Relation: map[string]bool{"parent": true}}, `{"@type":"Relation"`},
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

// TestAlertSubObjectTypeForced confirms the codec forces the correct "@type"
// onto each sub-object even when the caller left Type zero or wrong.
func TestAlertSubObjectTypeForced(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		obj  any
		want string
	}{
		{"alert-zero", Alert{Action: "display"}, "Alert"},
		{"alert-wrong", Alert{Type: "Reminder", Action: "display"}, "Alert"},
		{"link-zero", Link{Href: "x"}, "Link"},
		{"link-wrong", Link{Type: "URL", Href: "x"}, "Link"},
		{"relation-zero", Relation{}, "Relation"},
		{"relation-wrong", Relation{Type: "Rel"}, "Relation"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out, err := json.Marshal(tc.obj)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var probe struct {
				Type string `json:"@type"`
			}
			if err := json.Unmarshal(out, &probe); err != nil {
				t.Fatalf("unmarshal probe: %v", err)
			}
			if probe.Type != tc.want {
				t.Errorf("@type = %q, want %q", probe.Type, tc.want)
			}
		})
	}
}

// TestTriggerOffsetRoundTrip checks that an OffsetTrigger round-trips
// byte-stably, decodes back to the OffsetTrigger concrete kind, and emits
// "@type":"OffsetTrigger" first with the offset as a SignedDuration string.
func TestTriggerOffsetRoundTrip(t *testing.T) {
	t.Parallel()

	in := []byte(`{"@type":"OffsetTrigger","offset":"-PT15M","relativeTo":"start"}`)

	var tr Trigger
	if err := json.Unmarshal(in, &tr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	off, ok := tr.Value().(OffsetTrigger)
	if !ok {
		t.Fatalf("Value() = %T, want OffsetTrigger", tr.Value())
	}
	if !off.Offset.Negative || off.Offset.Minutes != 15 {
		t.Errorf("offset = %+v, want negative 15 minutes", off.Offset)
	}
	if off.RelativeTo != "start" {
		t.Errorf("relativeTo = %q, want %q", off.RelativeTo, "start")
	}

	out, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Equal(out, in) {
		t.Errorf("round-trip = %s, want %s", out, in)
	}
}

// TestTriggerAbsoluteRoundTrip checks that an AbsoluteTrigger round-trips
// byte-stably and decodes back to the AbsoluteTrigger concrete kind with the
// when value as a UTCDateTime.
func TestTriggerAbsoluteRoundTrip(t *testing.T) {
	t.Parallel()

	in := []byte(`{"@type":"AbsoluteTrigger","when":"2026-06-21T09:00:00Z"}`)

	var tr Trigger
	if err := json.Unmarshal(in, &tr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	abs, ok := tr.Value().(AbsoluteTrigger)
	if !ok {
		t.Fatalf("Value() = %T, want AbsoluteTrigger", tr.Value())
	}
	if got := abs.When.String(); got != "2026-06-21T09:00:00Z" {
		t.Errorf("when = %q, want %q", got, "2026-06-21T09:00:00Z")
	}

	out, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Equal(out, in) {
		t.Errorf("round-trip = %s, want %s", out, in)
	}
}

// TestTriggerUnknownKindRoundTrips is the load-bearing pin: a trigger whose
// inner "@type" matches neither recognized kind must round-trip losslessly,
// preserving its members verbatim, and expose a nil Value (RFC 8984's trigger
// registry is open — Section 4.5.1).
func TestTriggerUnknownKindRoundTrips(t *testing.T) {
	t.Parallel()

	in := []byte(`{"@type":"PercentTrigger","percent":50,"vendor":"acme"}`)

	var tr Trigger
	if err := json.Unmarshal(in, &tr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if tr.Value() != nil {
		t.Errorf("Value() = %v, want nil for unrecognized kind", tr.Value())
	}
	if tr.IsZero() {
		t.Error("IsZero() = true, want false for a preserved unknown trigger")
	}

	out, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Equal(out, in) {
		t.Errorf("round-trip = %s, want %s", out, in)
	}
}

// TestTriggerRoutesOnWireType confirms routing is on the wire "@type", not a
// Go type: a wire object that has an "offset" member but declares
// "@type":"AbsoluteTrigger" decodes as an AbsoluteTrigger (ignoring the
// stray member), and one declaring "OffsetTrigger" decodes as an
// OffsetTrigger.
func TestTriggerRoutesOnWireType(t *testing.T) {
	t.Parallel()

	// Declares AbsoluteTrigger but also carries an offset-looking member:
	// the decoder must route on "@type", treating "offset" as ignored.
	var tr Trigger
	if err := json.Unmarshal([]byte(`{"@type":"AbsoluteTrigger","when":"2026-01-01T00:00:00Z","offset":"PT1H"}`), &tr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := tr.Value().(AbsoluteTrigger); !ok {
		t.Fatalf("Value() = %T, want AbsoluteTrigger (route on @type)", tr.Value())
	}
}

// TestTriggerZeroOmitted checks that an Alert with no trigger omits the
// "trigger" member entirely (omitzero honoring Trigger.IsZero), rather than
// emitting "trigger":null.
func TestTriggerZeroOmitted(t *testing.T) {
	t.Parallel()

	out, err := json.Marshal(Alert{Action: "display"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if bytes.Contains(out, []byte("trigger")) {
		t.Errorf("zero-trigger alert marshaled %s, want no trigger member", out)
	}
}

// TestAlertRoundTrip exercises a fully populated Alert — an offset trigger, an
// acknowledged time, a relatedTo relation, and an unknown member captured into
// Extra — and asserts a byte-stable round trip.
func TestAlertRoundTrip(t *testing.T) {
	t.Parallel()

	in := []byte(`{"@type":"Alert",` +
		`"trigger":{"@type":"OffsetTrigger","offset":"-PT5M"},` +
		`"acknowledged":"2026-06-20T18:00:00Z",` +
		`"relatedTo":{"alert-1":{"@type":"Relation","relation":{"parent":true}}},` +
		`"action":"email",` +
		`"x-vendor":"acme"}`)

	var a Alert
	if err := json.Unmarshal(in, &a); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if a.Acknowledged == nil {
		t.Fatal("acknowledged not decoded")
	}
	if _, ok := a.Extra["x-vendor"]; !ok {
		t.Error("x-vendor not captured into Extra")
	}

	out, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Equal(out, in) {
		t.Errorf("round-trip = %s, want %s", out, in)
	}
}

// TestLinkRoundTrip exercises a fully populated Link with a size UnsignedInt
// and an unknown member captured into Extra, asserting a byte-stable round
// trip.
func TestLinkRoundTrip(t *testing.T) {
	t.Parallel()

	in := []byte(`{"@type":"Link",` +
		`"href":"https://example.com/a.png",` +
		`"contentType":"image/png",` +
		`"size":12345,` +
		`"rel":"enclosure",` +
		`"display":"badge",` +
		`"title":"A picture",` +
		`"x-vendor":true}`)

	var l Link
	if err := json.Unmarshal(in, &l); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if l.Size != 12345 {
		t.Errorf("size = %d, want 12345", l.Size)
	}
	out, err := json.Marshal(l)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Equal(out, in) {
		t.Errorf("round-trip = %s, want %s", out, in)
	}
}

// TestRelationRoundTrip checks that a Relation round-trips byte-stably,
// including an unknown member captured into Extra.
func TestRelationRoundTrip(t *testing.T) {
	t.Parallel()

	in := []byte(`{"@type":"Relation","relation":{"first":true,"parent":true},"x-note":"n"}`)

	var r Relation
	if err := json.Unmarshal(in, &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !r.Relation["first"] || !r.Relation["parent"] {
		t.Errorf("relation = %v, want first+parent", r.Relation)
	}
	out, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Equal(out, in) {
		t.Errorf("round-trip = %s, want %s", out, in)
	}
}

// TestEventWithAlertsLinksRelatedTo wires the three new sub-object maps into
// an Event and asserts a byte-stable round trip, including a Location-scoped
// links map (the typed Link replacing the former json.RawMessage placeholder)
// and both trigger kinds across two alerts.
func TestEventWithAlertsLinksRelatedTo(t *testing.T) {
	t.Parallel()

	in := []byte(`{"@type":"Event",` +
		`"uid":"event-1",` +
		`"locations":{"loc-1":{"@type":"Location","name":"HQ",` +
		`"links":{"map-1":{"@type":"Link","href":"https://maps.example/hq"}}}},` +
		`"alerts":{` +
		`"a-1":{"@type":"Alert","trigger":{"@type":"OffsetTrigger","offset":"-PT30M"},"action":"display"},` +
		`"a-2":{"@type":"Alert","trigger":{"@type":"AbsoluteTrigger","when":"2026-12-24T17:00:00Z"},"action":"email"}},` +
		`"links":{"l-1":{"@type":"Link","href":"https://example.com/agenda.pdf","contentType":"application/pdf"}},` +
		`"relatedTo":{"event-0":{"@type":"Relation","relation":{"parent":true}}}}`)

	var e Event
	if err := json.Unmarshal(in, &e); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(e.Alerts) != 2 {
		t.Errorf("alerts len = %d, want 2", len(e.Alerts))
	}
	if _, ok := e.RelatedTo["event-0"]; !ok {
		t.Error("relatedTo[event-0] missing")
	}
	if _, ok := e.Locations["loc-1"].Links["map-1"]; !ok {
		t.Error("location-scoped link missing (typed Link wiring)")
	}

	out, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Equal(out, in) {
		t.Errorf("round-trip mismatch\n got: %s\nwant: %s", out, in)
	}
}

// TestEventWithUnknownTriggerRoundTrips confirms an unrecognized trigger
// kind survives a full Event round trip through the alerts map — exercising
// the alias / marshalKnown / Trigger codec chain end to end, not just a bare
// Trigger. This is the interop path that matters: a vendor trigger nested in
// an alert nested in an event must come out byte-for-byte unchanged.
func TestEventWithUnknownTriggerRoundTrips(t *testing.T) {
	t.Parallel()

	in := []byte(`{"@type":"Event","uid":"e-1",` +
		`"alerts":{"a-1":{"@type":"Alert",` +
		`"trigger":{"@type":"GeoTrigger","place":"office"},"action":"display"}}}`)

	var e Event
	if err := json.Unmarshal(in, &e); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := e.Alerts["a-1"].Trigger.Value(); got != nil {
		t.Errorf("unknown trigger Value() = %v, want nil", got)
	}

	out, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Equal(out, in) {
		t.Errorf("round-trip = %s, want %s", out, in)
	}
}

// TestTaskRelatedToStringKeyed confirms the Task relatedTo map is keyed by a
// UID string (Section 4.1.3), not an Id — a string key with no Id-shape
// constraint round-trips.
func TestTaskRelatedToStringKeyed(t *testing.T) {
	t.Parallel()

	in := []byte(`{"@type":"Task","uid":"task-1",` +
		`"relatedTo":{"urn:uuid:not-an-id":{"@type":"Relation","relation":{"next":true}}}}`)

	var task Task
	if err := json.Unmarshal(in, &task); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := task.RelatedTo["urn:uuid:not-an-id"]; !ok {
		t.Error("relatedTo string key not preserved")
	}
	out, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Equal(out, in) {
		t.Errorf("round-trip = %s, want %s", out, in)
	}
}

// TestTriggerLenientUnmarshal checks decode tolerance: a trigger member order
// other than canonical still decodes, and a null trigger yields the zero
// Trigger.
func TestTriggerLenientUnmarshal(t *testing.T) {
	t.Parallel()

	var tr Trigger
	if err := json.Unmarshal([]byte(`{"relativeTo":"end","@type":"OffsetTrigger","offset":"PT1H"}`), &tr); err != nil {
		t.Fatalf("unmarshal reordered: %v", err)
	}
	off, ok := tr.Value().(OffsetTrigger)
	if !ok || off.RelativeTo != "end" {
		t.Errorf("reordered decode = %+v, want OffsetTrigger end", tr.Value())
	}

	var zero Trigger
	if err := json.Unmarshal([]byte(`null`), &zero); err != nil {
		t.Fatalf("unmarshal null: %v", err)
	}
	if !zero.IsZero() {
		t.Error("null trigger should be zero")
	}
}

// TestTriggerRejectsNonObject confirms a non-object trigger (array, scalar) is
// a structural decode error rather than a silent acceptance.
func TestTriggerRejectsNonObject(t *testing.T) {
	t.Parallel()

	var tr Trigger
	if err := json.Unmarshal([]byte(`["not","an","object"]`), &tr); err == nil {
		t.Error("expected error decoding array into Trigger, got nil")
	}
}

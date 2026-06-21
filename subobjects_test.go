// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// This file tests the Participant, Location, and VirtualLocation sub-objects
// (RFC 8984, Sections 4.4.6, 4.2.5, 4.2.6) and their wiring into the Event
// and Task "participants", "locations", and "virtualLocations" maps. The
// behavior mirrors the top-level types: "@type" emitted first and forced,
// order-tolerant lenient decode, and lossless byte-stable round-tripping of
// unknown members through Extra.

// TestSubObjectTypeEmittedFirst checks that each sub-object marshals its
// mandatory "@type" member first (RFC 8984, Section 1.4.1). The codec relies
// on Type being the first declared field; this guards a future reorder.
func TestSubObjectTypeEmittedFirst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		obj  any
		want string
	}{
		{"participant", Participant{Name: "Tom"}, `{"@type":"Participant"`},
		{"location", Location{Name: "HQ"}, `{"@type":"Location"`},
		{"virtualLocation", VirtualLocation{URI: "tel:+1-555"}, `{"@type":"VirtualLocation"`},
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

// TestSubObjectTypeForced confirms the codec forces the correct "@type" onto
// a sub-object even when the caller left Type zero or set it to a wrong
// value, matching the top-level types' normalization.
func TestSubObjectTypeForced(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		obj  any
		want string
	}{
		{"participant-zero", Participant{Name: "Tom"}, "Participant"},
		{"participant-wrong", Participant{Type: "wrong", Name: "Tom"}, "Participant"},
		{"location-zero", Location{Name: "HQ"}, "Location"},
		{"location-wrong", Location{Type: "Place", Name: "HQ"}, "Location"},
		{"virtualLocation-zero", VirtualLocation{URI: "x"}, "VirtualLocation"},
		{"virtualLocation-wrong", VirtualLocation{Type: "Online", URI: "x"}, "VirtualLocation"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out, err := json.Marshal(tc.obj)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var got struct {
				Type string `json:"@type"`
			}
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if got.Type != tc.want {
				t.Errorf("@type = %q, want %q", got.Type, tc.want)
			}
		})
	}
}

// TestSubObjectRoundTrip checks decode-then-encode reproduces the canonical
// input bytes for each sub-object, including the byte-stable preservation of
// unknown members via Extra and the sorted-key ordering the codec produces.
func TestSubObjectRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		kind  string // "participant" | "location" | "virtualLocation"
	}{
		{
			name:  "participant-full",
			input: `{"@type":"Participant","name":"Tom Tool","email":"tom@example.com","sendTo":{"imip":"mailto:tom@example.com"},"roles":{"attendee":true},"participationStatus":"accepted","expectReply":true}`,
			kind:  "participant",
		},
		{
			name:  "participant-delegation",
			input: `{"@type":"Participant","name":"Group","delegatedTo":{"id-a":true,"id-b":true},"memberOf":{"grp-1":true}}`,
			kind:  "participant",
		},
		{
			name:  "participant-unknown-member",
			input: `{"@type":"Participant","name":"Tom","example.com:rank":42}`,
			kind:  "participant",
		},
		{
			name:  "location-relativeTo",
			input: `{"@type":"Location","name":"NRT","relativeTo":"end","timeZone":"Asia/Tokyo"}`,
			kind:  "location",
		},
		{
			name:  "location-coordinates",
			input: `{"@type":"Location","name":"The Music Bowl","coordinates":"geo:40.7829,-73.9654"}`,
			kind:  "location",
		},
		{
			name:  "location-unknown-member",
			input: `{"@type":"Location","name":"HQ","x-floor":3}`,
			kind:  "location",
		},
		{
			name:  "virtualLocation-features",
			input: `{"@type":"VirtualLocation","name":"Bridge","uri":"https://example.com/x","features":{"audio":true,"video":true}}`,
			kind:  "virtualLocation",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got []byte
			switch tc.kind {
			case "participant":
				got = reencode[Participant](t, tc.input)
			case "location":
				got = reencode[Location](t, tc.input)
			case "virtualLocation":
				got = reencode[VirtualLocation](t, tc.input)
			default:
				t.Fatalf("unknown kind %q", tc.kind)
			}
			if string(got) != tc.input {
				t.Errorf("round trip not byte-stable\n got: %s\nwant: %s", got, tc.input)
			}
		})
	}
}

// TestSubObjectDecodeTolerant confirms member order is ignored and a missing
// "@type" is accepted on decode (the codec's lenient posture), with Type
// normalized afterward.
func TestSubObjectDecodeTolerant(t *testing.T) {
	t.Parallel()

	t.Run("participant-reordered-no-type", func(t *testing.T) {
		t.Parallel()
		var p Participant
		if err := json.Unmarshal([]byte(`{"email":"a@b.example","name":"A"}`), &p); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if p.Type != "Participant" {
			t.Errorf("Type = %q, want Participant", p.Type)
		}
		if p.Name != "A" || p.Email != "a@b.example" {
			t.Errorf("decoded = %+v, want Name=A Email=a@b.example", p)
		}
	})

	t.Run("location-non-object", func(t *testing.T) {
		t.Parallel()
		var l Location
		if err := json.Unmarshal([]byte(`["not","an","object"]`), &l); err == nil {
			t.Error("decode of a JSON array into Location succeeded, want error")
		}
	})
}

// TestSubObjectExtraTypedAccess checks an unknown member survives the round
// trip and is reachable through DecodeJSON, the typed-extension accessor.
func TestSubObjectExtraTypedAccess(t *testing.T) {
	t.Parallel()

	var loc Location
	if err := json.Unmarshal([]byte(`{"@type":"Location","name":"HQ","example.com:floor":7}`), &loc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var floor int
	if err := DecodeJSON(loc.Extra["example.com:floor"], &floor); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if floor != 7 {
		t.Errorf("floor = %d, want 7", floor)
	}
}

// TestEventSubObjectMapsTyped is the JSCAL-18 acceptance assertion: the
// corpus figures that carry sub-objects populate the TYPED Event maps
// (Participants, Locations, VirtualLocations) rather than dropping them into
// the open-extension Extra map. It parses the figures via the package's
// public Parse entry point and inspects the decoded Event.
func TestEventSubObjectMapsTyped(t *testing.T) {
	t.Parallel()

	read := func(t *testing.T, name string) *Event {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(rfc8984CorpusDir, name))
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		obj, err := Parse(raw)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		ev, ok := obj.(*Event)
		if !ok {
			t.Fatalf("Parse returned %T, want *Event", obj)
		}
		// None of the sub-object property names should leak into Extra.
		for _, k := range []string{"participants", "locations", "virtualLocations"} {
			if _, found := ev.Extra[k]; found {
				t.Errorf("%q landed in Extra, want a typed field", k)
			}
		}
		return ev
	}

	t.Run("6.6-locations", func(t *testing.T) {
		t.Parallel()
		ev := read(t, "6.6-event-with-end-time-zone.json")
		if len(ev.Locations) != 2 {
			t.Fatalf("Locations = %d, want 2", len(ev.Locations))
		}
		// Keys are stable identifiers preserved verbatim.
		end, ok := ev.Locations["2"]
		if !ok {
			t.Fatalf("location key %q missing; got keys %v", "2", locationKeys(ev))
		}
		if end.Name != "Narita International Airport (NRT)" {
			t.Errorf("location 2 Name = %q", end.Name)
		}
		if end.RelativeTo != "end" {
			t.Errorf("location 2 RelativeTo = %q, want end", end.RelativeTo)
		}
		if end.TimeZone != "Asia/Tokyo" {
			t.Errorf("location 2 TimeZone = %q, want Asia/Tokyo", end.TimeZone)
		}
	})

	t.Run("6.8-location-and-virtualLocation", func(t *testing.T) {
		t.Parallel()
		ev := read(t, "6.8-multiple-locations-localization.json")
		if len(ev.Locations) != 1 {
			t.Errorf("Locations = %d, want 1", len(ev.Locations))
		}
		if len(ev.VirtualLocations) != 1 {
			t.Errorf("VirtualLocations = %d, want 1", len(ev.VirtualLocations))
		}
		loc, ok := ev.Locations["c0503d30-8c50-4372-87b5-7657e8e0fedd"]
		if !ok {
			t.Fatalf("location key missing; got %v", locationKeys(ev))
		}
		if loc.Coordinates != "geo:40.7829,-73.9654" {
			t.Errorf("Coordinates = %q", loc.Coordinates)
		}
		vloc, ok := ev.VirtualLocations["vloc1"]
		if !ok {
			t.Fatal("virtualLocation key \"vloc1\" missing")
		}
		if vloc.URI != "https://stream.example.com/the_band_2020" {
			t.Errorf("VirtualLocation URI = %q", vloc.URI)
		}
		// locale and localizations are not typed here; they stay in Extra.
		for _, k := range []string{"locale", "localizations"} {
			if _, found := ev.Extra[k]; !found {
				t.Errorf("%q missing from Extra; phase-4 leftover should round-trip there", k)
			}
		}
	})

	t.Run("6.10-participants-and-virtualLocation", func(t *testing.T) {
		t.Parallel()
		ev := read(t, "6.10-recurring-event-with-participants.json")
		if len(ev.Participants) != 2 {
			t.Fatalf("Participants = %d, want 2", len(ev.Participants))
		}
		if len(ev.VirtualLocations) != 1 {
			t.Errorf("VirtualLocations = %d, want 1", len(ev.VirtualLocations))
		}
		tom, ok := ev.Participants["dG9tQGZvb2Jhci5xlLmNvbQ"]
		if !ok {
			t.Fatalf("participant key missing; got %v", participantKeys(ev))
		}
		if tom.Name != "Tom Tool" || tom.Email != "tom@foobar.example.com" {
			t.Errorf("Tom = %+v", tom)
		}
		if tom.ParticipationStatus != "accepted" {
			t.Errorf("Tom ParticipationStatus = %q, want accepted", tom.ParticipationStatus)
		}
		if !tom.Roles["attendee"] {
			t.Errorf("Tom Roles = %v, want attendee:true", tom.Roles)
		}
		if tom.SendTo["imip"] != "mailto:tom@calendar.example.com" {
			t.Errorf("Tom SendTo[imip] = %q", tom.SendTo["imip"])
		}
		// replyTo is not modeled in this phase; it must round-trip via Extra.
		if _, found := ev.Extra["replyTo"]; !found {
			t.Error("replyTo missing from Extra; it should round-trip there")
		}
	})
}

// locationKeys and participantKeys return the map keys for error messages.
func locationKeys(ev *Event) []Id {
	keys := make([]Id, 0, len(ev.Locations))
	for k := range ev.Locations {
		keys = append(keys, k)
	}
	return keys
}

func participantKeys(ev *Event) []Id {
	keys := make([]Id, 0, len(ev.Participants))
	for k := range ev.Participants {
		keys = append(keys, k)
	}
	return keys
}

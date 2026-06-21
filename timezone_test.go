// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"bytes"
	"encoding/json"
	"testing"
)

// This file tests the TimeZone and TimeZoneRule sub-objects (RFC 8984,
// Sections 4.7.2 and 4.7.3) and their wiring into the Event and Task
// "localizations" and embedded "timeZones" maps (Sections 4.6.1 and 4.7.1).
// The behavior mirrors the other sub-objects: "@type" emitted first and
// forced, order-tolerant lenient decode, and lossless byte-stable
// round-tripping of unknown members through Extra.

// TestTimeZoneTypeEmittedFirst checks that TimeZone and TimeZoneRule marshal
// their mandatory "@type" member first (RFC 8984, Section 1.4.1). The codec
// relies on Type being the first declared field; this guards a future reorder.
func TestTimeZoneTypeEmittedFirst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		obj  any
		want string
	}{
		{"timeZone", TimeZone{TzID: "/Example/Custom"}, `{"@type":"TimeZone"`},
		{"timeZoneRule", TimeZoneRule{OffsetTo: "+01:00"}, `{"@type":"TimeZoneRule"`},
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

// TestTimeZoneTypeForced confirms the codec forces the correct "@type" onto a
// TimeZone or TimeZoneRule even when the caller left Type zero or set it to a
// wrong value, matching the other sub-objects' normalization.
func TestTimeZoneTypeForced(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		obj  any
		want string
	}{
		{"timeZone-zero", TimeZone{TzID: "/X"}, "TimeZone"},
		{"timeZone-wrong", TimeZone{Type: "VTimeZone", TzID: "/X"}, "TimeZone"},
		{"timeZoneRule-zero", TimeZoneRule{OffsetTo: "+00:00"}, "TimeZoneRule"},
		{"timeZoneRule-wrong", TimeZoneRule{Type: "STANDARD", OffsetTo: "+00:00"}, "TimeZoneRule"},
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

// TestTimeZoneRoundTrip checks decode-then-encode reproduces the canonical
// input bytes for TimeZone and TimeZoneRule, including the byte-stable
// preservation of unknown members via Extra and the sorted-key ordering the
// codec produces.
func TestTimeZoneRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		kind  string // "timeZone" | "timeZoneRule"
	}{
		{
			name:  "timeZoneRule-standard",
			input: `{"@type":"TimeZoneRule","start":"2020-10-25T03:00:00","offsetFrom":"+02:00","offsetTo":"+01:00","names":{"CET":true}}`,
			kind:  "timeZoneRule",
		},
		{
			name:  "timeZoneRule-recurring",
			input: `{"@type":"TimeZoneRule","start":"1970-03-29T02:00:00","offsetFrom":"+01:00","offsetTo":"+02:00","recurrenceRules":[{"@type":"RecurrenceRule","frequency":"yearly","byDay":[{"@type":"NDay","day":"su","nthOfPeriod":-1}],"byMonth":["3"]}],"names":{"CEST":true}}`,
			kind:  "timeZoneRule",
		},
		{
			name:  "timeZoneRule-unknown-member",
			input: `{"@type":"TimeZoneRule","offsetTo":"+00:00","example.com:source":"tzdb"}`,
			kind:  "timeZoneRule",
		},
		{
			name:  "timeZone-full",
			input: `{"@type":"TimeZone","tzId":"/Example/Custom","updated":"2021-01-01T00:00:00Z","url":"https://tz.example.com/custom","validUntil":"2030-01-01T00:00:00Z","aliases":{"Example/Old":true},"standard":[{"@type":"TimeZoneRule","start":"2020-10-25T03:00:00","offsetFrom":"+02:00","offsetTo":"+01:00","names":{"CET":true}}],"daylight":[{"@type":"TimeZoneRule","start":"2020-03-29T02:00:00","offsetFrom":"+01:00","offsetTo":"+02:00","names":{"CEST":true}}]}`,
			kind:  "timeZone",
		},
		{
			name:  "timeZone-unknown-member",
			input: `{"@type":"TimeZone","tzId":"/X","example.com:origin":"manual"}`,
			kind:  "timeZone",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got []byte
			switch tc.kind {
			case "timeZone":
				got = reencode[TimeZone](t, tc.input)
			case "timeZoneRule":
				got = reencode[TimeZoneRule](t, tc.input)
			default:
				t.Fatalf("unknown kind %q", tc.kind)
			}
			if string(got) != tc.input {
				t.Errorf("round trip not byte-stable\n got: %s\nwant: %s", got, tc.input)
			}
		})
	}
}

// TestTimeZoneDecodeTolerant confirms member order is ignored and a missing
// "@type" is accepted on decode (the codec's lenient posture), with Type
// normalized afterward, and that a non-object input is rejected.
func TestTimeZoneDecodeTolerant(t *testing.T) {
	t.Parallel()

	t.Run("rule-reordered-no-type", func(t *testing.T) {
		t.Parallel()
		var r TimeZoneRule
		if err := json.Unmarshal([]byte(`{"offsetTo":"+01:00","offsetFrom":"+00:00"}`), &r); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if r.Type != "TimeZoneRule" {
			t.Errorf("Type = %q, want TimeZoneRule", r.Type)
		}
		if r.OffsetFrom != "+00:00" || r.OffsetTo != "+01:00" {
			t.Errorf("decoded = %+v", r)
		}
	})

	t.Run("timeZone-non-object", func(t *testing.T) {
		t.Parallel()
		var z TimeZone
		if err := json.Unmarshal([]byte(`"not an object"`), &z); err == nil {
			t.Error("decode of a JSON string into TimeZone succeeded, want error")
		}
	})
}

// TestEventCustomTimeZoneRoundTrip is the JSCAL-20 acceptance assertion: an
// Event that carries a custom, "/"-prefixed timeZone defined in its embedded
// timeZones map (with both a standard and a daylight rule) and a localizations
// entry round-trips byte-stably through the codec, with both the timeZones and
// localizations properties landing on their typed fields rather than in Extra.
//
//nolint:funlen // a single end-to-end construction is clearer than splitting it.
func TestEventCustomTimeZoneRoundTrip(t *testing.T) {
	t.Parallel()

	ev := &Event{
		UID:      "tz-demo",
		Title:    "Clocks go forward",
		Start:    mustLocalDateTime(t, "2020-03-29T01:30:00"),
		TimeZone: "/Example/Berlin",
		TimeZones: map[TimeZoneId]TimeZone{
			"/Example/Berlin": {
				TzID: "/Example/Berlin",
				Standard: []TimeZoneRule{{
					Start:      mustLocalDateTime(t, "2020-10-25T03:00:00"),
					OffsetFrom: "+02:00",
					OffsetTo:   "+01:00",
					Names:      map[string]bool{"CET": true},
				}},
				Daylight: []TimeZoneRule{{
					Start:      mustLocalDateTime(t, "2020-03-29T02:00:00"),
					OffsetFrom: "+01:00",
					OffsetTo:   "+02:00",
					RecurrenceRules: []RecurrenceRule{{
						Frequency: FrequencyYearly,
						ByMonth:   []string{"3"},
					}},
					Names: map[string]bool{"CEST": true},
				}},
			},
		},
		Localizations: map[string]PatchObject{
			"de": {
				"title": json.RawMessage(`"Sommerzeit-Umstellung"`),
			},
		},
	}

	out, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Decode the marshaled bytes and confirm the typed fields are populated,
	// not the open-extension Extra map.
	got, err := Parse(out)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	dec, ok := got.(*Event)
	if !ok {
		t.Fatalf("Parse returned %T, want *Event", got)
	}
	for _, k := range []string{"timeZones", "localizations"} {
		if _, found := dec.Extra[k]; found {
			t.Errorf("%q landed in Extra, want a typed field", k)
		}
	}
	zone, ok := dec.TimeZones["/Example/Berlin"]
	if !ok {
		t.Fatal("custom timeZone \"/Example/Berlin\" missing from TimeZones")
	}
	if len(zone.Standard) != 1 || len(zone.Daylight) != 1 {
		t.Fatalf("zone rules = standard %d / daylight %d, want 1 / 1",
			len(zone.Standard), len(zone.Daylight))
	}
	if zone.Daylight[0].OffsetTo != "+02:00" {
		t.Errorf("daylight OffsetTo = %q, want +02:00", zone.Daylight[0].OffsetTo)
	}
	if len(zone.Daylight[0].RecurrenceRules) != 1 {
		t.Errorf("daylight RecurrenceRules = %d, want 1", len(zone.Daylight[0].RecurrenceRules))
	}
	if _, found := dec.Localizations["de"]; !found {
		t.Errorf("localization \"de\" missing; got %v", localizationKeys(dec))
	}

	// The custom zone the event references resolves in its own timeZones map —
	// the closure the validation phase will enforce holds for this object.
	resolved := TimeZoneId(ev.TimeZone).ResolvesIn(func(k TimeZoneId) bool {
		_, found := ev.TimeZones[k]
		return found
	})
	if !resolved {
		t.Error("event timeZone does not resolve in its own timeZones map")
	}

	// Byte-stable: a decode-then-encode of the marshaled bytes reproduces them
	// exactly.
	round, err := json.Marshal(dec)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if !bytes.Equal(round, out) {
		t.Errorf("round trip not byte-stable\n got: %s\nwant: %s", round, out)
	}
}

// TestTaskTimeZonesAndLocalizations confirms the typed timeZones and
// localizations fields are wired onto Task as well as Event and round-trip
// byte-stably.
func TestTaskTimeZonesAndLocalizations(t *testing.T) {
	t.Parallel()

	input := `{"@type":"Task","uid":"task-1","localizations":{"fr":{"title":"Tâche"}},"timeZones":{"/Custom":{"@type":"TimeZone","tzId":"/Custom","standard":[{"@type":"TimeZoneRule","start":"2020-01-01T00:00:00","offsetFrom":"+00:00","offsetTo":"+00:00"}]}}}`

	got := reencode[Task](t, input)
	if string(got) != input {
		t.Errorf("round trip not byte-stable\n got: %s\nwant: %s", got, input)
	}

	var task Task
	if err := json.Unmarshal([]byte(input), &task); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(task.TimeZones) != 1 {
		t.Errorf("TimeZones = %d, want 1", len(task.TimeZones))
	}
	if len(task.Localizations) != 1 {
		t.Errorf("Localizations = %d, want 1", len(task.Localizations))
	}
}

// localizationKeys returns a localizations map's keys for error messages.
func localizationKeys(ev *Event) []string {
	keys := make([]string, 0, len(ev.Localizations))
	for k := range ev.Localizations {
		keys = append(keys, k)
	}
	return keys
}

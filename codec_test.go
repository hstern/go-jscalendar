// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestTypeAutoSetOnMarshal verifies the codec's defining behavior: the
// "@type" discriminator (RFC 8984, Section 1.4.1) is emitted with the
// correct constant value even when the caller left the Type field at its
// zero value. A consumer constructing an object in Go should not have to
// remember to set Type by hand; the marshaler fills it in.
func TestTypeAutoSetOnMarshal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		obj  any
		want string
	}{
		{"event zero type", &Event{UID: "e1"}, `{"@type":"Event","uid":"e1"}`},
		{"task zero type", &Task{UID: "t1"}, `{"@type":"Task","uid":"t1"}`},
		{"group zero type", &Group{UID: "g1"}, `{"@type":"Group","uid":"g1"}`},
		// A value receiver (not a pointer) must marshal identically — the
		// MarshalJSON method set has to cover both forms.
		{"event value receiver", Event{UID: "e1"}, `{"@type":"Event","uid":"e1"}`},
		{"task value receiver", Task{UID: "t1"}, `{"@type":"Task","uid":"t1"}`},
		{"group value receiver", Group{UID: "g1"}, `{"@type":"Group","uid":"g1"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out, err := json.Marshal(tc.obj)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(out) != tc.want {
				t.Errorf("marshal = %s, want %s", out, tc.want)
			}
		})
	}
}

// TestTypeFirstMember pins the interop-stability requirement that "@type"
// is the FIRST member of the marshaled object for every top-level type
// (RFC 8984, Section 1.4.1), regardless of how the object was built.
func TestTypeFirstMember(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		obj  any
		want string
	}{
		{"event", &Event{UID: "e1", Title: "t"}, `{"@type":"Event"`},
		{"task", &Task{UID: "t1", Title: "t"}, `{"@type":"Task"`},
		{"group", &Group{UID: "g1", Title: "t"}, `{"@type":"Group"`},
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

// TestWrongTypeValueNormalized checks that a caller-supplied Type that
// disagrees with the concrete Go type is overwritten with the correct
// discriminator on marshal. The Go type is the source of truth for the
// "@type" value; a stale or mistyped Type field must not leak onto the
// wire.
func TestWrongTypeValueNormalized(t *testing.T) {
	t.Parallel()

	ev := Event{Type: "Task", UID: "e1"}
	out, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"@type":"Event","uid":"e1"}`
	if string(out) != want {
		t.Errorf("marshal = %s, want %s", out, want)
	}
}

// TestOrderTolerantDecode checks the tolerant-unmarshal posture (Postel's
// law): members may arrive in any order, and "@type" need not come first
// on the wire. The decoded object re-marshals to canonical, byte-stable
// output with "@type" first.
func TestOrderTolerantDecode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
		kind string
	}{
		{
			name: "event type last",
			raw:  `{"uid":"e1","title":"Sync","@type":"Event"}`,
			want: `{"@type":"Event","uid":"e1","title":"Sync"}`,
			kind: "event",
		},
		{
			name: "task scrambled",
			raw:  `{"title":"Report","@type":"Task","uid":"t1","priority":5}`,
			want: `{"@type":"Task","uid":"t1","title":"Report","priority":5}`,
			kind: "task",
		},
		{
			name: "group type middle",
			raw:  `{"uid":"g1","@type":"Group","title":"Cal"}`,
			want: `{"@type":"Group","uid":"g1","title":"Cal"}`,
			kind: "group",
		},
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
			if string(got) != tc.want {
				t.Errorf("round-trip = %s, want %s", got, tc.want)
			}
		})
	}
}

// TestDecodeTypeAbsentTolerated checks that a missing "@type" member does
// not cause the decoder to reject the input — strictness (requiring
// "@type") is the validation phase's job, not the codec's. On re-marshal
// the codec supplies the correct "@type" from the concrete Go type.
func TestDecodeTypeAbsentTolerated(t *testing.T) {
	t.Parallel()

	const raw = `{"uid":"e1","title":"No type"}`
	var ev Event
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		t.Fatalf("unmarshal without @type: %v", err)
	}
	if ev.UID != "e1" {
		t.Errorf("UID = %q, want %q", ev.UID, "e1")
	}

	out, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"@type":"Event","uid":"e1","title":"No type"}`
	if string(out) != want {
		t.Errorf("marshal = %s, want %s", out, want)
	}
}

// TestDecodeRejectsNonObject checks that a non-object JSON value — array,
// scalar, or null — is rejected on decode. A top-level JSCalendar object
// is always a JSON object; this is a structural decode error, distinct
// from the lenient handling of object members.
func TestDecodeRejectsNonObject(t *testing.T) {
	t.Parallel()

	inputs := []string{`[]`, `"Event"`, `42`, `true`, `null`}
	for _, raw := range inputs {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			var ev Event
			if err := json.Unmarshal([]byte(raw), &ev); err == nil {
				t.Errorf("unmarshal %s: want error, got nil", raw)
			}
		})
	}
}

// TestByteStableAcrossCycles checks that decode→encode is idempotent: a
// canonical input survives an arbitrary number of round-trips unchanged.
// Byte stability is the property interop consumers depend on when pinning
// exact request/response bytes.
func TestByteStableAcrossCycles(t *testing.T) {
	t.Parallel()

	const canonical = `{"@type":"Event","uid":"e1","title":"Sync",` +
		`"keywords":{"a":true,"b":true},"start":"2020-01-15T13:00:00",` +
		`"duration":"PT1H","timeZone":"America/New_York"}`

	cur := canonical
	for i := range 5 {
		got := reencode[Event](t, cur)
		if string(got) != canonical {
			t.Fatalf("cycle %d: round-trip = %s, want %s", i, got, canonical)
		}
		cur = string(got)
	}
}

// TestZeroValueMarshalsWellFormed checks that marshaling a fully zero
// object — no UID, no other members — still produces well-formed JSON that
// leads with the "@type" discriminator. This is the degenerate input to
// the marshaler and guards the path where every optional field is
// suppressed by omitempty and only the forced discriminator remains.
func TestZeroValueMarshalsWellFormed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		obj  any
		want string
	}{
		{Event{}, `{"@type":"Event"}`},
		{Task{}, `{"@type":"Task"}`},
		{Group{}, `{"@type":"Group"}`},
	}

	for _, tc := range tests {
		out, err := json.Marshal(tc.obj)
		if err != nil {
			t.Fatalf("marshal %T: %v", tc.obj, err)
		}
		if !json.Valid(out) {
			t.Errorf("marshal %T produced invalid JSON: %s", tc.obj, out)
		}
		if string(out) != tc.want {
			t.Errorf("marshal %T = %s, want %s", tc.obj, out, tc.want)
		}
	}
}

// FuzzEventCodec exercises the Event codec against arbitrary input bytes.
// It pins two properties the codec must hold for any input, not just the
// curated cases above:
//
//   - Never panic. Decoding adversarial or malformed bytes must return an
//     error, never crash — the codec is on the trust boundary for any
//     consumer parsing untrusted JSCalendar data.
//   - Marshal idempotence. Once an input decodes successfully, re-encoding
//     and decoding again reaches a fixed point: the second marshal equals
//     the first. This is the byte-stability guarantee, checked against
//     inputs no hand-written table would enumerate.
func FuzzEventCodec(f *testing.F) {
	f.Add(`{"@type":"Event","uid":"e1"}`)
	f.Add(`{"uid":"e1","@type":"Event","title":"x"}`)
	f.Add(`{}`)
	f.Add(`not json`)
	f.Add(`[1,2,3]`)
	f.Add(`{"start":"2020-01-15T13:00:00","duration":"PT1H"}`)
	// Seeds carrying unknown members exercise the open-extension path
	// (capture into Extra, splice back on marshal); the idempotence
	// property below must hold for them too.
	f.Add(`{"@type":"Event","uid":"e1","x-vendor":{"k":[1,2]}}`)
	f.Add(`{"uid":"e1","futureProp":42,"@type":"Event","x-null":null}`)

	f.Fuzz(func(t *testing.T, input string) {
		var ev Event
		if err := json.Unmarshal([]byte(input), &ev); err != nil {
			return // malformed input is allowed to fail; it must not panic
		}

		first, err := json.Marshal(ev)
		if err != nil {
			t.Fatalf("marshal after successful decode: %v", err)
		}

		var ev2 Event
		if err := json.Unmarshal(first, &ev2); err != nil {
			t.Fatalf("re-decode of own output failed: %v\noutput: %s", err, first)
		}
		second, err := json.Marshal(ev2)
		if err != nil {
			t.Fatalf("second marshal: %v", err)
		}
		if !bytes.Equal(first, second) {
			t.Errorf("marshal not idempotent:\nfirst:  %s\nsecond: %s", first, second)
		}
	})
}

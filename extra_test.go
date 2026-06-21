// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

// TestUnknownMembersRoundTripLossless is the defining test for the
// open-extension mechanism: an object carrying members the model does not
// recognize re-marshals byte-for-byte to the canonical form, with "@type"
// first, the known properties next, and the unknown members last in sorted
// key order. A vendor extension and a future-spec property are
// indistinguishable on the wire, so both must survive the round trip.
func TestUnknownMembersRoundTripLossless(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		kind string
		raw  string
		want string
	}{
		{
			name: "event with scalar and object extensions",
			kind: "event",
			raw: `{"@type":"Event","uid":"e1","title":"Sync",` +
				`"example.com:mood":"happy",` +
				`"x-vendor":{"nested":[1,2,3],"flag":true}}`,
			want: `{"@type":"Event","uid":"e1","title":"Sync",` +
				`"example.com:mood":"happy",` +
				`"x-vendor":{"nested":[1,2,3],"flag":true}}`,
		},
		{
			name: "task with extension sorted after known",
			kind: "task",
			raw: `{"@type":"Task","uid":"t1","zzExtension":1,` +
				`"aaExtension":2,"priority":5}`,
			want: `{"@type":"Task","uid":"t1","priority":5,` +
				`"aaExtension":2,"zzExtension":1}`,
		},
		{
			name: "group with extension",
			kind: "group",
			raw:  `{"@type":"Group","uid":"g1","futureProp":{"x":1}}`,
			want: `{"@type":"Group","uid":"g1","futureProp":{"x":1}}`,
		},
		{
			name: "extension carrying explicit null",
			kind: "event",
			raw:  `{"@type":"Event","uid":"e1","x-null":null}`,
			want: `{"@type":"Event","uid":"e1","x-null":null}`,
		},
		{
			// Byte stability is canonical-stable, not verbatim: whitespace
			// inside an extension value is compacted on the first marshal,
			// the same normalization the codec applies to known fields. The
			// member order inside the object is preserved (a RawMessage is
			// opaque; only whitespace is stripped).
			name: "extension value whitespace canonicalized",
			kind: "event",
			raw:  `{"@type":"Event","uid":"e1","x-obj":{ "b" : 1 , "a" : 2 }}`,
			want: `{"@type":"Event","uid":"e1","x-obj":{"b":1,"a":2}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := reencodeExtra(t, tc.kind, tc.raw)
			if string(got) != tc.want {
				t.Errorf("round-trip = %s, want %s", got, tc.want)
			}
		})
	}
}

// TestExtraPreservesNumericPrecision is the regression guard for the design
// decision behind Extra: storing extension values as [json.RawMessage]
// rather than map[string]any. A large integer (beyond float64's exact range)
// and a high-precision decimal must survive byte-for-byte; routing them
// through float64 — what a map[string]any decode would do — would reformat
// both and lose precision.
func TestExtraPreservesNumericPrecision(t *testing.T) {
	t.Parallel()

	const raw = `{"@type":"Event","uid":"e1",` +
		`"x-bigint":100000000000000000000,` +
		`"x-decimal":0.30000000000000004}`
	got := reencodeExtra(t, "event", raw)
	if string(got) != raw {
		t.Errorf("round-trip = %s, want %s (numeric bytes must survive verbatim)", got, raw)
	}
}

// TestExtraCapturedExactly checks that unknown members land in Extra with
// their exact wire bytes and that known members never do — a member whose
// name matches a known property is decoded into that property, not captured.
func TestExtraCapturedExactly(t *testing.T) {
	t.Parallel()

	const raw = `{"@type":"Event","uid":"e1","title":"Sync",` +
		`"x-vendor":{"a":1},"example:n":42}`
	var ev Event
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Known members decoded into their fields.
	if ev.UID != "e1" || ev.Title != "Sync" {
		t.Errorf("known fields = {UID:%q Title:%q}, want {e1 Sync}", ev.UID, ev.Title)
	}
	// Known member names never leak into Extra.
	for _, known := range []string{"@type", "uid", "title"} {
		if _, ok := ev.Extra[known]; ok {
			t.Errorf("Extra unexpectedly contains known member %q", known)
		}
	}
	// Unknown members captured verbatim.
	want := map[string]string{"x-vendor": `{"a":1}`, "example:n": `42`}
	if len(ev.Extra) != len(want) {
		t.Fatalf("Extra has %d members, want %d: %v", len(ev.Extra), len(want), ev.Extra)
	}
	for k, v := range want {
		if got := string(ev.Extra[k]); got != v {
			t.Errorf("Extra[%q] = %s, want %s", k, got, v)
		}
	}
}

// TestNoExtraLeavesFieldNil checks that an object with only known members
// decodes to a nil Extra map, not an empty non-nil one. This keeps the zero
// value meaningful and avoids surprising a caller that ranges over Extra.
func TestNoExtraLeavesFieldNil(t *testing.T) {
	t.Parallel()

	const raw = `{"@type":"Event","uid":"e1","title":"Sync"}`
	var ev Event
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ev.Extra != nil {
		t.Errorf("Extra = %v, want nil", ev.Extra)
	}

	// And it marshals with no trailing splice artifacts.
	out, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != raw {
		t.Errorf("marshal = %s, want %s", out, raw)
	}
}

// TestExtraByteStableAcrossCycles checks that an object with extensions is a
// fixed point of decode→encode once in canonical form, the property interop
// consumers rely on when pinning exact bytes.
func TestExtraByteStableAcrossCycles(t *testing.T) {
	t.Parallel()

	const canonical = `{"@type":"Task","uid":"t1","priority":5,` +
		`"a-ext":[1,2],"b-ext":{"k":"v"},"c-ext":"text"}`
	cur := canonical
	for i := range 5 {
		got := reencodeExtra(t, "task", cur)
		if string(got) != canonical {
			t.Fatalf("cycle %d: = %s, want %s", i, got, canonical)
		}
		cur = string(got)
	}
}

// TestMarshalProgrammaticExtra checks that an Extra map populated in Go (not
// via a decode) is emitted after the known properties in sorted key order.
func TestMarshalProgrammaticExtra(t *testing.T) {
	t.Parallel()

	ev := Event{
		UID: "e1",
		Extra: map[string]json.RawMessage{
			"z-ext": json.RawMessage(`"last"`),
			"a-ext": json.RawMessage(`1`),
		},
	}
	out, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"@type":"Event","uid":"e1","a-ext":1,"z-ext":"last"}`
	if string(out) != want {
		t.Errorf("marshal = %s, want %s", out, want)
	}
}

// TestMarshalDropsKnownNameCollision checks the strict-marshal guard: an
// Extra member whose name collides with a known property is dropped so the
// output never carries a duplicate JSON member. The known property wins; the
// output stays valid JSON regardless of how the caller populated Extra.
func TestMarshalDropsKnownNameCollision(t *testing.T) {
	t.Parallel()

	ev := Event{
		UID: "real-uid",
		Extra: map[string]json.RawMessage{
			"uid":   json.RawMessage(`"shadow-uid"`),
			"@type": json.RawMessage(`"Task"`),
			"x-ok":  json.RawMessage(`true`),
		},
	}
	out, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !json.Valid(out) {
		t.Fatalf("marshal produced invalid JSON: %s", out)
	}
	const want = `{"@type":"Event","uid":"real-uid","x-ok":true}`
	if string(out) != want {
		t.Errorf("marshal = %s, want %s", out, want)
	}
}

// TestMarshalDoesNotMutateExtra checks that the collision filter does not
// mutate the caller's Extra map — filtering must operate on a copy.
func TestMarshalDoesNotMutateExtra(t *testing.T) {
	t.Parallel()

	extra := map[string]json.RawMessage{
		"uid":  json.RawMessage(`"shadow"`),
		"x-ok": json.RawMessage(`1`),
	}
	ev := Event{UID: "e1", Extra: extra}
	if _, err := json.Marshal(ev); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(extra) != 2 {
		t.Errorf("Extra mutated: now %v", extra)
	}
}

// TestExtraAcrossAllTypes checks that the mechanism behaves identically for
// Event, Task, and Group — the field, the capture, and the splice are
// uniform across the three top-level types.
func TestExtraAcrossAllTypes(t *testing.T) {
	t.Parallel()

	kinds := []struct {
		kind string
		raw  string
	}{
		{"event", `{"@type":"Event","uid":"e1","x-ext":7}`},
		{"task", `{"@type":"Task","uid":"t1","x-ext":7}`},
		{"group", `{"@type":"Group","uid":"g1","x-ext":7}`},
	}
	for _, tc := range kinds {
		t.Run(tc.kind, func(t *testing.T) {
			t.Parallel()
			got := reencodeExtra(t, tc.kind, tc.raw)
			if string(got) != tc.raw {
				t.Errorf("round-trip = %s, want %s", got, tc.raw)
			}
		})
	}
}

// TestDecodeJSON checks the typed-access helper: a present value decodes,
// and a nil or empty value reports ErrExtensionAbsent without touching v.
func TestDecodeJSON(t *testing.T) {
	t.Parallel()

	type loc struct {
		Name string `json:"name"`
	}

	t.Run("present", func(t *testing.T) {
		t.Parallel()
		var got loc
		if err := DecodeJSON(json.RawMessage(`{"name":"HQ"}`), &got); err != nil {
			t.Fatalf("DecodeJSON: %v", err)
		}
		if got.Name != "HQ" {
			t.Errorf("Name = %q, want HQ", got.Name)
		}
	})

	t.Run("absent values report ErrExtensionAbsent", func(t *testing.T) {
		t.Parallel()
		for _, raw := range []json.RawMessage{nil, {}, json.RawMessage("")} {
			var got loc
			err := DecodeJSON(raw, &got)
			if !errors.Is(err, ErrExtensionAbsent) {
				t.Errorf("DecodeJSON(%q) error = %v, want ErrExtensionAbsent", raw, err)
			}
			if got.Name != "" {
				t.Errorf("v mutated on absent value: %+v", got)
			}
		}
	})

	t.Run("missing map key is absent", func(t *testing.T) {
		t.Parallel()
		ev := Event{}
		var got loc
		if err := DecodeJSON(ev.Extra["nope"], &got); !errors.Is(err, ErrExtensionAbsent) {
			t.Errorf("error = %v, want ErrExtensionAbsent", err)
		}
	})

	t.Run("explicit null is present and a no-op", func(t *testing.T) {
		t.Parallel()
		got := loc{Name: "keep"}
		if err := DecodeJSON(json.RawMessage(`null`), &got); err != nil {
			t.Fatalf("DecodeJSON(null): %v", err)
		}
		if got.Name != "keep" {
			t.Errorf("null overwrote v: %+v", got)
		}
	})
}

// TestEncodeJSONRoundTrip checks that EncodeJSON and DecodeJSON compose into
// an identity for a typed value, and that a value stored via EncodeJSON
// survives a full object marshal/unmarshal cycle.
func TestEncodeJSONRoundTrip(t *testing.T) {
	t.Parallel()

	type loc struct {
		Name string `json:"name"`
		Rank int    `json:"rank"`
	}
	in := loc{Name: "HQ", Rank: 3}

	raw, err := EncodeJSON(in)
	if err != nil {
		t.Fatalf("EncodeJSON: %v", err)
	}

	var back loc
	if err := DecodeJSON(raw, &back); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if back != in {
		t.Errorf("round-trip = %+v, want %+v", back, in)
	}

	// Store on an object, marshal, unmarshal, read back the typed value.
	ev := Event{UID: "e1", Extra: map[string]json.RawMessage{"example:loc": raw}}
	wire, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded Event
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	var fromObj loc
	if err := DecodeJSON(decoded.Extra["example:loc"], &fromObj); err != nil {
		t.Fatalf("DecodeJSON from object: %v", err)
	}
	if fromObj != in {
		t.Errorf("from object = %+v, want %+v", fromObj, in)
	}
}

// TestKnownMemberNames pins the reflection helper that drives both capture
// and the collision filter: every json-tagged property name is reported,
// the Extra field (json:"-") is excluded, and "@type" is present so it is
// never treated as an extension.
func TestKnownMemberNames(t *testing.T) {
	t.Parallel()

	names := knownMemberNames(reflect.TypeOf(eventAlias{}))
	for _, want := range []string{"@type", "uid", "title", "start", "timeZone"} {
		if _, ok := names[want]; !ok {
			t.Errorf("known names missing %q", want)
		}
	}
	if _, ok := names["Extra"]; ok {
		t.Error(`known names must not include the Extra field (tagged json:"-")`)
	}
	if _, ok := names["-"]; ok {
		t.Error(`known names must not include "-"`)
	}
}

// reencodeExtra decodes raw into the named top-level type and re-marshals it,
// returning the canonical wire bytes. It is the extension-aware analogue of
// the reencode helper in codec_test.go.
func reencodeExtra(t *testing.T, kind, raw string) []byte {
	t.Helper()
	switch kind {
	case "event":
		return reencode[Event](t, raw)
	case "task":
		return reencode[Task](t, raw)
	case "group":
		return reencode[Group](t, raw)
	default:
		t.Fatalf("unknown kind %q", kind)
		return nil
	}
}

// reencode is defined in codec_test.go and shared across the codec tests.

// ExampleEncodeJSON shows attaching a typed vendor extension to an Event and
// reading it back after a round trip through JSON.
func ExampleEncodeJSON() {
	type geo struct {
		Lat, Lon float64
	}

	raw, err := EncodeJSON(geo{Lat: 40.7, Lon: -74.0})
	if err != nil {
		panic(err)
	}
	ev := Event{UID: "e1", Extra: map[string]json.RawMessage{"example.com:geo": raw}}

	wire, err := json.Marshal(ev)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(wire))

	var decoded Event
	if err := json.Unmarshal(wire, &decoded); err != nil {
		panic(err)
	}
	var g geo
	if err := DecodeJSON(decoded.Extra["example.com:geo"], &g); err != nil {
		panic(err)
	}
	fmt.Printf("%.1f,%.1f\n", g.Lat, g.Lon)
	// Output:
	// {"@type":"Event","uid":"e1","example.com:geo":{"Lat":40.7,"Lon":-74}}
	// 40.7,-74.0
}

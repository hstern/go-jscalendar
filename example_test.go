// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar_test

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hstern/go-jscalendar"
)

// These examples mirror the README quickstart. They live in the external
// jscalendar_test package so they read exactly as a consumer would write them,
// importing the package by its public path. Each demonstrates one step of the
// quickstart: parse-and-marshal, validate, and the typed open-extension
// pattern. The iCalendar conversion has its own example in the ical
// sub-package.

// ExampleParse_quickstart decodes a JSCalendar object whose concrete type is
// named by its "@type" discriminator, reads a few typed fields, and marshals it
// back out. The codec emits "@type" first and is byte-stable, so the round trip
// is deterministic.
func ExampleParse_quickstart() {
	const src = `{
	  "@type": "Event",
	  "uid": "a8df3f1e-1c2b-4d5e-9f00-112233445566",
	  "title": "Team sync",
	  "start": "2026-07-01T09:00:00",
	  "timeZone": "America/New_York",
	  "duration": "PT1H"
	}`

	obj, err := jscalendar.Parse([]byte(src))
	if err != nil {
		panic(err)
	}

	// Parse routes on "@type": an Event yields a *jscalendar.Event.
	ev, ok := obj.(*jscalendar.Event)
	if !ok {
		panic(fmt.Sprintf("unexpected type %T", obj))
	}
	fmt.Printf("%s in %s for %s\n", ev.Title, ev.TimeZone, ev.Duration)

	// Marshal back out: "@type" emits first, output is byte-stable.
	out, err := json.Marshal(ev)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(out))

	// Output:
	// Team sync in America/New_York for PT1H
	// {"@type":"Event","uid":"a8df3f1e-1c2b-4d5e-9f00-112233445566","title":"Team sync","start":"2026-07-01T09:00:00","duration":"PT1H","timeZone":"America/New_York"}
}

// ExampleEvent_Validate runs the opt-in, strict validation pass over an Event.
// Decoding stays lenient (Postel's law); Validate is where the RFC 8984 §4–§5
// MUSTs are enforced. A violation is reported as a *jscalendar.ValidationError.
func ExampleEvent_Validate() {
	// An Event with no "uid" violates RFC 8984 §4.1.2.
	ev := &jscalendar.Event{Title: "Missing UID"}

	err := ev.Validate()
	var verr *jscalendar.ValidationError
	if errors.As(err, &verr) {
		fmt.Printf("invalid %s\n", verr.Property)
	}

	// Supplying the required uid satisfies the MUST.
	ev.UID = "event-1"
	fmt.Println("valid:", ev.Validate() == nil)

	// Output:
	// invalid uid
	// valid: true
}

// ExampleDecodeJSON reads an unknown ("open-extension") member out of an
// Event's Extra map into a typed value. Unknown members round-trip losslessly
// as json.RawMessage and are decoded only on demand.
func ExampleDecodeJSON() {
	const src = `{
	  "@type": "Event",
	  "uid": "event-1",
	  "example.com/room": {"building": "HQ", "floor": 3}
	}`

	obj, err := jscalendar.Parse([]byte(src))
	if err != nil {
		panic(err)
	}
	ev := obj.(*jscalendar.Event)

	type room struct {
		Building string `json:"building"`
		Floor    int    `json:"floor"`
	}
	var r room
	if err := jscalendar.DecodeJSON(ev.Extra["example.com/room"], &r); err != nil {
		panic(err)
	}
	fmt.Printf("%s floor %d\n", r.Building, r.Floor)

	// An absent member is reported with ErrExtensionAbsent, distinct from a
	// member present on the wire as JSON null.
	err = jscalendar.DecodeJSON(ev.Extra["example.com/missing"], &r)
	fmt.Println("absent:", errors.Is(err, jscalendar.ErrExtensionAbsent))

	// Output:
	// HQ floor 3
	// absent: true
}

// ExampleEncodeJSON_setExtra sets an open-extension member on an Event via the
// Extra map. EncodeJSON marshals a typed value into the json.RawMessage the
// codec splices back onto the wire, after the known properties and in sorted
// key order.
func ExampleEncodeJSON_setExtra() {
	ev := &jscalendar.Event{UID: "event-1", Title: "Team sync"}

	raw, err := jscalendar.EncodeJSON(map[string]string{"building": "HQ"})
	if err != nil {
		panic(err)
	}
	ev.Extra = map[string]json.RawMessage{"example.com/room": raw}

	out, err := json.Marshal(ev)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(out))

	// Output:
	// {"@type":"Event","uid":"event-1","title":"Team sync","example.com/room":{"building":"HQ"}}
}

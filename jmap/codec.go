// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jmap

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/hstern/go-jscalendar"
)

// This file defines the JSON codec for [CalendarEvent]. A CalendarEvent is, on
// the wire, a single JSON object: the JSCalendar Event members and the JMAP
// additional members live side by side in one flat object (the spec layers the
// JMAP properties directly onto the Event, it does not nest them). The codec
// preserves the core jscalendar package's two contracts:
//
//   - "@type":"Event" leads the object. The embedded Event's own codec already
//     emits "@type" first and byte-stably; this codec keeps that lead by
//     splicing the JMAP members into the Event's marshaled bytes after "@type"
//     and the Event's known/Extra members rather than prepending them.
//   - Unknown members round-trip losslessly. Any member that is neither a JMAP
//     addition nor a known JSCalendar property flows into the embedded Event's
//     Extra map (json.RawMessage), exactly as it would on a bare Event, so a
//     vendor extension or a future-spec property survives a round trip.
//
// Byte-stability. The output is deterministic: the embedded Event marshals
// byte-stably (known members in declaration order, Extra in sorted key order),
// and the JMAP members are appended in a fixed declaration order via the
// jmapOnly struct. The same input therefore always produces the same bytes.

// MarshalJSON encodes the CalendarEvent as one JSON object: the embedded
// JSCalendar [jscalendar.Event] (with "@type":"Event" first and its Extra
// members preserved), followed by the JMAP-specific members in declaration
// order. The result is byte-stable.
//
// A nil embedded Event is treated as an empty Event for marshaling, so the
// output is still a valid object carrying "@type":"Event" plus whatever JMAP
// members are set — a CalendarEvent is a JSCalendar Event by definition, and
// the discriminator belongs on the wire regardless.
func (ce CalendarEvent) MarshalJSON() ([]byte, error) {
	ev := ce.Event
	if ev == nil {
		ev = &jscalendar.Event{}
	}
	eventBytes, err := json.Marshal(ev)
	if err != nil {
		return nil, fmt.Errorf("jmap: marshal CalendarEvent event: %w", err)
	}

	jmapBytes, err := json.Marshal(ce.jmapView())
	if err != nil {
		return nil, fmt.Errorf("jmap: marshal CalendarEvent members: %w", err)
	}
	// jmapBytes is "{...}" — "{}" when every JMAP member is at its zero value.
	if len(jmapBytes) <= 2 {
		return eventBytes, nil
	}

	// eventBytes ends with '}', jmapBytes is '{...}' with at least one member;
	// drop eventBytes' closing brace and jmapBytes' opening brace, joined by a
	// comma, to merge the two objects into one flat object.
	out := make([]byte, 0, len(eventBytes)+len(jmapBytes))
	out = append(out, eventBytes[:len(eventBytes)-1]...)
	out = append(out, ',')
	out = append(out, jmapBytes[1:]...)
	return out, nil
}

// UnmarshalJSON decodes a JSON object into the CalendarEvent. It splits the
// object's members into the JMAP additions (decoded onto the CalendarEvent's
// own fields) and everything else (decoded into a fresh embedded
// [jscalendar.Event] via the core codec, which captures unknown members into
// the Event's Extra map).
//
// Decoding is tolerant in the same way the core codec is: member order does not
// matter, a missing or mismatched "@type" is accepted, and unrecognized members
// are preserved rather than rejected. Strict checks — that calendarIds is
// present and non-empty, that a draft was not un-drafted, that method is absent
// — belong to the transport layer, not here.
func (ce *CalendarEvent) UnmarshalJSON(data []byte) error {
	if !isJSONObject(data) {
		return fmt.Errorf("jmap: decode CalendarEvent: expected a JSON object")
	}

	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return fmt.Errorf("jmap: decode CalendarEvent: %w", err)
	}

	// Partition the members: JMAP additions go to the jmapOnly decode, the rest
	// (the JSCalendar Event members, including any unknown extension members) go
	// to the embedded Event so its codec can route them to known fields or Extra.
	jmapMembers := make(map[string]json.RawMessage, len(jmapMemberNames))
	eventMembers := make(map[string]json.RawMessage, len(all))
	for name, raw := range all {
		if _, ok := jmapMemberNames[name]; ok {
			jmapMembers[name] = raw
			continue
		}
		eventMembers[name] = raw
	}

	var jv jmapOnly
	if len(jmapMembers) > 0 {
		jmapBytes, err := json.Marshal(jmapMembers)
		if err != nil {
			return fmt.Errorf("jmap: decode CalendarEvent members: %w", err)
		}
		if err := json.Unmarshal(jmapBytes, &jv); err != nil {
			return fmt.Errorf("jmap: decode CalendarEvent members: %w", err)
		}
	}

	eventBytes, err := json.Marshal(eventMembers)
	if err != nil {
		return fmt.Errorf("jmap: decode CalendarEvent event: %w", err)
	}
	var ev jscalendar.Event
	if err := json.Unmarshal(eventBytes, &ev); err != nil {
		return fmt.Errorf("jmap: decode CalendarEvent event: %w", err)
	}

	*ce = CalendarEvent{Event: &ev}
	ce.setJMAP(jv)
	return nil
}

// isJSONObject reports whether data is a JSON object — its first significant
// byte is '{'. It guards the decoder against a non-object top-level value
// (array, scalar, or null) with a clear error, mirroring the core codec's
// structural check.
func isJSONObject(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	return len(trimmed) > 0 && trimmed[0] == '{'
}

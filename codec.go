// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
)

// The "@type" discriminator values for the three top-level object types
// (RFC 8984, Section 4.1.1). The codec forces these onto the wire and onto
// the decoded value, so the discriminator is always present and correct
// regardless of how the object was built.
const (
	typeEvent = "Event"
	typeTask  = "Task"
	typeGroup = "Group"
)

// This file defines the JSON codec for the three top-level JSCalendar
// object types — [Event], [Task], and [Group]. It replaces the default
// struct-tag encoding referenced in the marshaling note in types.go with a
// dedicated codec that honors two RFC 8984 requirements the default
// encoding satisfied only incidentally:
//
//   - The "@type" discriminator (Section 1.4.1) is emitted as the FIRST
//     member of the object and is always present with the correct value,
//     even when the caller left the Type field at its zero value. This is
//     the interop-stability pattern the wider suite uses: a fixed,
//     leading discriminator so a consumer can route on the first bytes
//     without buffering the whole object.
//   - Decoding is tolerant (Postel's law): members may arrive in any
//     order, "@type" need not lead or even be present, and unrecognized
//     members do not cause rejection. Strict checks — that "@type" is
//     present and matches, that required members exist — belong to the
//     opt-in validation phase, not here.
//
// Marshaling remains byte-stable: known properties emit in struct
// declaration order, which encoding/json preserves, so a decoded object
// re-marshals to deterministic bytes.
//
// Implementation shape. Each type's MarshalJSON sets the discriminator and
// then encodes through a type alias (eventAlias, taskAlias, groupAlias)
// that has the same fields and tags but NONE of the methods of the named
// type. Encoding the alias therefore uses encoding/json's default struct
// codec rather than recursing into MarshalJSON. UnmarshalJSON decodes
// through the same alias for an order-tolerant, lenient decode.
//
// Open-extension seam. Unknown members are preserved losslessly — a vendor
// extension and a future-spec property look identical on the wire, and both
// must round-trip — via the Extra field (map[string]json.RawMessage) on each
// top-level type. The seam is the marshalKnown / unmarshalKnown boundary
// below: marshalKnown produces the bytes for the known fields with "@type"
// first, then splices the Extra members into that byte stream (between the
// known members and the closing brace, in sorted key order); unmarshalKnown
// decodes the known fields and captures any member it did not consume into
// Extra. Keeping that splice in one place is why the alias encode/decode is
// funneled through these two helpers rather than inlined per type. The splice
// and capture mechanics live in extra.go, alongside the DecodeJSON /
// EncodeJSON accessors.

// These aliases carry the field set and struct tags of their named types
// but none of the methods, so encoding/json applies its default struct
// codec to them instead of recursing into the custom MarshalJSON /
// UnmarshalJSON. They are the mechanism by which the codec delegates the
// per-field encoding while retaining control of the "@type" member.
type (
	eventAlias Event
	taskAlias  Task
	groupAlias Group
)

// marshalKnown encodes the known struct fields of a top-level object with
// the "@type" discriminator forced to typeName and emitted first.
//
// The alias argument must be the method-stripped alias of the named type
// (eventAlias for Event, etc.) with its Type field already set to
// typeName, so the default struct encoder emits "@type" first (it is the
// first declared field and now always non-empty despite the omitempty
// tag).
//
// This is the open-extension seam: extra holds the object's unknown
// members, which spliceExtra appends to the byte stream this function
// returns, just before the closing brace and in sorted key order,
// preserving "@type"-first order and the known-field ordering. A member in
// extra whose name collides with a known property is dropped (the known
// property is authoritative), so the output is always valid JSON regardless
// of how a caller populated the Extra map.
func marshalKnown(alias any, typeName string, extra map[string]json.RawMessage) ([]byte, error) {
	known, err := json.Marshal(alias)
	if err != nil {
		return nil, fmt.Errorf("jscalendar: marshal %s: %w", typeName, err)
	}
	return spliceExtra(known, extra, reflect.TypeOf(alias), typeName)
}

// unmarshalKnown decodes data into the method-stripped alias pointed to by
// aliasPtr using encoding/json's default, order-tolerant struct decoder,
// then forces the named type's discriminator onto the result.
//
// Decoding is lenient: any JSON object is accepted, members may be in any
// order, a missing or mismatched "@type" is tolerated (validation's job),
// and unrecognized members are ignored. A non-object input (array, scalar,
// or null) is a structural error, since a top-level object is always a
// JSON object.
//
// setType assigns the discriminator on the decoded value; it is passed
// separately because the alias is handled as an opaque any here. This is
// the decode side of the open-extension seam: captureExtra collects the
// members this decoder does not map to a known field, which the caller
// stores in the object's Extra field. The returned map is nil when the
// input carried no unknown members.
func unmarshalKnown(data []byte, aliasPtr any, typeName string, setType func()) (map[string]json.RawMessage, error) {
	if !isJSONObject(data) {
		return nil, fmt.Errorf("jscalendar: decode %s: expected a JSON object", typeName)
	}
	if err := json.Unmarshal(data, aliasPtr); err != nil {
		return nil, fmt.Errorf("jscalendar: decode %s: %w", typeName, err)
	}
	extra, err := captureExtra(data, reflect.TypeOf(aliasPtr).Elem())
	if err != nil {
		return nil, fmt.Errorf("jscalendar: decode %s: %w", typeName, err)
	}
	setType()
	return extra, nil
}

// MarshalJSON encodes the Event with "@type":"Event" as the first member
// and the remaining known properties in declaration order, followed by any
// [Event.Extra] members in sorted key order, yielding byte-stable output.
// The discriminator is set automatically: a zero or mismatched [Event.Type]
// is normalized to "Event".
func (e Event) MarshalJSON() ([]byte, error) {
	a := eventAlias(e)
	a.Type = typeEvent
	return marshalKnown(a, typeEvent, e.Extra)
}

// UnmarshalJSON decodes a JSON object into the Event. It is tolerant of
// member order and of a missing "@type"; strict checks belong to the
// validation phase. [Event.Type] is set to "Event" after a successful
// decode regardless of the wire value. Members with no corresponding known
// property are captured into [Event.Extra] for lossless round-tripping.
func (e *Event) UnmarshalJSON(data []byte) error {
	var a eventAlias
	extra, err := unmarshalKnown(data, &a, typeEvent, func() { a.Type = typeEvent })
	if err != nil {
		return err
	}
	*e = Event(a)
	e.Extra = extra
	return nil
}

// MarshalJSON encodes the Task with "@type":"Task" as the first member,
// then the known properties in declaration order and any [Task.Extra]
// members in sorted key order, byte-stable throughout. A zero or mismatched
// [Task.Type] is normalized to "Task".
func (t Task) MarshalJSON() ([]byte, error) {
	a := taskAlias(t)
	a.Type = typeTask
	return marshalKnown(a, typeTask, t.Extra)
}

// UnmarshalJSON decodes a JSON object into the Task, tolerant of member
// order and a missing "@type". [Task.Type] is set to "Task" after a
// successful decode regardless of the wire value. Members with no
// corresponding known property are captured into [Task.Extra].
func (t *Task) UnmarshalJSON(data []byte) error {
	var a taskAlias
	extra, err := unmarshalKnown(data, &a, typeTask, func() { a.Type = typeTask })
	if err != nil {
		return err
	}
	*t = Task(a)
	t.Extra = extra
	return nil
}

// MarshalJSON encodes the Group with "@type":"Group" as the first member,
// then the known properties in declaration order and any [Group.Extra]
// members in sorted key order, byte-stable throughout. A zero or mismatched
// [Group.Type] is normalized to "Group". The Entries members are emitted
// verbatim as raw JSON, preserving each member's exact bytes.
func (g Group) MarshalJSON() ([]byte, error) {
	a := groupAlias(g)
	a.Type = typeGroup
	return marshalKnown(a, typeGroup, g.Extra)
}

// UnmarshalJSON decodes a JSON object into the Group, tolerant of member
// order and a missing "@type". [Group.Type] is set to "Group" after a
// successful decode regardless of the wire value. Each Entries member is
// retained as raw JSON; members with no corresponding known property are
// captured into [Group.Extra].
func (g *Group) UnmarshalJSON(data []byte) error {
	var a groupAlias
	extra, err := unmarshalKnown(data, &a, typeGroup, func() { a.Type = typeGroup })
	if err != nil {
		return err
	}
	*g = Group(a)
	g.Extra = extra
	return nil
}

// isJSONObject reports whether data is a JSON object — its first
// significant byte is '{'. It is a cheap structural guard so the codec can
// reject a non-object top-level value (array, scalar, or null) with a
// clear error before handing the bytes to encoding/json, which would
// otherwise report a less specific type-mismatch error.
func isJSONObject(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	return len(trimmed) > 0 && trimmed[0] == '{'
}

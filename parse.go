// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"encoding/json"
	"errors"
	"fmt"
)

// This file defines the top-level decode entry point [Parse], which routes a
// JSCalendar object to the concrete Go type named by its "@type" discriminator
// (RFC 8984, Section 1.4.1), and the [Group.Entry] accessor, which applies the
// same routing to a Group's lazily retained entries.
//
// Parse is the inverse of the codec's [Event.MarshalJSON] / [Task.MarshalJSON]
// / [Group.MarshalJSON]: those force "@type" onto the wire; Parse reads it back
// to decide which type to construct. Unlike the per-type UnmarshalJSON methods —
// which are deliberately lenient and tolerate a missing or mismatched "@type"
// because a caller who wrote `var e Event` has already chosen the type — Parse
// has no such caller-supplied type to fall back on. The discriminator is the
// only signal, so an absent or unrecognized "@type" is a typed error rather
// than a silent default: returning, say, an empty *Event for `{"uid":"x"}`
// would invent a type the wire never claimed.

// UnknownTypeError reports that [Parse] (or [Group.Entry]) could not route a
// JSCalendar object because its "@type" discriminator was absent or named a
// type the parser does not construct.
//
// Type is the offending discriminator value: the empty string when the "@type"
// member was absent (or present as JSON null), or the verbatim string when it
// was present but unrecognized. The two cases are distinguished by [Absent] so
// a caller can tell "no @type at all" from "an @type we don't handle" without
// string-matching the message.
type UnknownTypeError struct {
	// Type is the verbatim "@type" value that could not be routed, or the
	// empty string when no "@type" member was present.
	Type string
	// Absent reports whether the "@type" member was missing entirely (true)
	// rather than present with an unrecognized value (false). When Absent is
	// true, Type is always the empty string.
	Absent bool
}

// Error implements the error interface.
func (e *UnknownTypeError) Error() string {
	if e.Absent {
		return "jscalendar: cannot parse object: no \"@type\" member"
	}
	return fmt.Sprintf("jscalendar: cannot parse object: unknown \"@type\" %q", e.Type)
}

// Parse decodes a single top-level JSCalendar object and returns it as the
// concrete type named by its "@type" discriminator (RFC 8984, Section 1.4.1):
// an "@type" of "Event" yields a *[Event], "Task" yields a *[Task], and "Group"
// yields a *[Group]. The returned any always holds one of those three pointer
// types on success.
//
// The discriminator is mandatory here. An object whose "@type" member is absent
// (or JSON null), or whose value is none of the three recognized strings, is
// rejected with a *[UnknownTypeError] — Parse never guesses a default type for
// an object the wire did not label. This is stricter than the per-type
// UnmarshalJSON methods, which tolerate a missing "@type" because the caller
// already chose the target type by declaring it; Parse has only the wire to go
// on.
//
// Decoding of the routed type is otherwise the lenient decode of that type's
// UnmarshalJSON: member order is ignored and unknown members round-trip through
// the type's Extra map. A non-object input (array, scalar, or null) is a
// structural error, not an UnknownTypeError, since the "@type" peek only applies
// to an object.
func Parse(b []byte) (any, error) {
	typ, absent, err := peekType(b)
	if err != nil {
		return nil, err
	}
	switch typ {
	case typeEvent:
		return decodeInto[Event](b)
	case typeTask:
		return decodeInto[Task](b)
	case typeGroup:
		return decodeInto[Group](b)
	default:
		return nil, &UnknownTypeError{Type: typ, Absent: absent}
	}
}

// Entry decodes the Group entry at index i and returns it as the concrete type
// named by its "@type" discriminator — a *[Event] or a *[Task] — using the same
// routing as [Parse]. A Group's [Group.Entries] are retained verbatim as
// [json.RawMessage] (lazy decode) so the exact member bytes round-trip; Entry is
// the accessor that turns one of those raw entries into a typed value on demand.
//
// The index must be in range: i < len(g.Entries). An out-of-range index is a
// programming error and panics, matching slice-index conventions; call
// [Group.NumEntries] (or take len(g.Entries) directly) to bound the loop.
//
// An entry whose "@type" is absent or unrecognized — including a nested "Group",
// which the spec permits as an entry but which this accessor does not construct
// because Group entries are constrained to Event and Task (RFC 8984, Section
// 5.3.1) — yields a *[UnknownTypeError]. A "Group" entry is therefore reported
// as an unknown type rather than silently decoded, surfacing a malformed corpus
// instead of hiding it.
func (g *Group) Entry(i int) (any, error) {
	raw := g.Entries[i]
	typ, absent, err := peekType(raw)
	if err != nil {
		return nil, err
	}
	switch typ {
	case typeEvent:
		return decodeInto[Event](raw)
	case typeTask:
		return decodeInto[Task](raw)
	default:
		return nil, &UnknownTypeError{Type: typ, Absent: absent}
	}
}

// decodeInto unmarshals b into a freshly allocated T and returns it as a
// pointer wrapped in any, the shared tail of every [Parse] and [Group.Entry]
// dispatch arm. T is one of the three top-level types, whose UnmarshalJSON
// performs the lenient decode; pulling the allocate-decode-return triple into a
// generic helper keeps each switch arm a single line and removes the repetition
// the per-type bodies would otherwise carry.
func decodeInto[T Event | Task | Group](b []byte) (any, error) {
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// NumEntries reports the number of entries in the Group, the bound for a
// [Group.Entry] loop. It is len(g.Entries) and exists so a caller iterating the
// entries reads as a pair with Entry without reaching into the raw slice.
func (g *Group) NumEntries() int {
	return len(g.Entries)
}

// peekType reads the "@type" member of a JSON object without decoding the rest
// of it. It returns the discriminator string and whether the member was absent.
//
// The peek decodes only the one member it needs: a struct with a single
// "@type"-tagged field, so the remaining (and possibly large) object body is
// skipped by encoding/json rather than materialized. A present-but-null "@type"
// is reported as absent (empty string, absent true), matching the spec's
// treatment of a null member as no member. A non-object input is a structural
// error returned verbatim so the caller can distinguish it from the typed
// UnknownTypeError that an absent discriminator produces.
func peekType(b []byte) (typ string, absent bool, err error) {
	if !isJSONObject(b) {
		return "", false, errors.New("jscalendar: cannot parse object: expected a JSON object")
	}
	var probe struct {
		Type *string `json:"@type"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		return "", false, fmt.Errorf("jscalendar: cannot parse object: %w", err)
	}
	if probe.Type == nil {
		return "", true, nil
	}
	return *probe.Type, false, nil
}

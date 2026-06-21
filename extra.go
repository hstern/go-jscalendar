// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// This file implements the open-extension mechanism that lets the three
// top-level types — [Event], [Task], and [Group] — round-trip object
// members they do not model as known properties. Each type carries an
// Extra field of type map[string]json.RawMessage; the codec in codec.go
// captures unknown members into it on decode and splices them back onto the
// wire on encode. The design rationale (RFC 8984): a vendor extension and a
// property from a future revision of the spec are indistinguishable on the
// wire, so both must be preserved verbatim rather than discarded.
//
// json.RawMessage — not map[string]any — is the value type on purpose:
//
//   - Byte-stable round-trip. The captured bytes re-marshal deterministically
//     (map keys sorted by encoding/json), where map[string]any would reorder
//     nested object keys and lose number formatting.
//   - Zero deserialize cost for members the consumer never reads.
//   - No any in the public surface: a consumer that wants a typed view opts
//     in explicitly via DecodeJSON.

// ErrExtensionAbsent is returned by [DecodeJSON] when the supplied
// [json.RawMessage] is nil or empty, the convention for "this extension
// member is not present." It lets a caller distinguish an absent member
// from a member that is present but holds the JSON value null (which decodes
// without error), using [errors.Is].
var ErrExtensionAbsent = errors.New("jscalendar: extension value absent")

// DecodeJSON unmarshals an extension value into v, a thin wrapper over
// [json.Unmarshal] for typed access to a member stored in an Extra map.
//
// A nil or empty raw is treated as "extension absent": v is left unchanged
// and the error is [ErrExtensionAbsent], so a missing map key (whose lookup
// yields the nil zero value) and an explicitly empty value are handled
// identically. Reading a present member is therefore:
//
//	var loc MyLocation
//	if err := jscalendar.DecodeJSON(ev.Extra["example.com:location"], &loc); err != nil {
//		// errors.Is(err, jscalendar.ErrExtensionAbsent) reports a missing member.
//	}
//
// A member present on the wire as JSON null is not absent: raw is "null",
// which [json.Unmarshal] accepts as a no-op, leaving v unchanged with a nil
// error.
func DecodeJSON(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		return ErrExtensionAbsent
	}
	return json.Unmarshal(raw, v)
}

// EncodeJSON marshals v into a [json.RawMessage] suitable for storing as an
// extension value in an Extra map, a thin wrapper over [json.Marshal]:
//
//	raw, err := jscalendar.EncodeJSON(MyLocation{Name: "HQ"})
//	if err != nil {
//		// handle
//	}
//	ev.Extra = map[string]json.RawMessage{"example.com:location": raw}
//
// The returned bytes are compacted by [json.Marshal]; the codec re-marshals
// Extra values on encode regardless, so storing already-compact bytes keeps
// the round-trip byte-stable.
func EncodeJSON(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// captureExtra returns the object members in data that do not correspond to
// a known JSON property of the struct type t. data must already be a JSON
// object (the codec checks this before calling). The result is nil when
// every member maps to a known property, so an object with no extensions
// leaves the Extra field nil rather than an empty non-nil map.
//
// The "@type" discriminator is a known member of every top-level type, so it
// is never captured here.
func captureExtra(data []byte, t reflect.Type) (map[string]json.RawMessage, error) {
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, nil
	}
	for name := range knownMemberNames(t) {
		delete(all, name)
	}
	if len(all) == 0 {
		return nil, nil
	}
	return all, nil
}

// spliceExtra appends the extra members to known — the already-marshaled
// bytes of a top-level object's known properties — just before the closing
// brace, in the sorted key order [json.Marshal] produces for a map. known
// is always a non-empty JSON object (it carries at least the forced "@type"
// member), so it ends in '}' and has at least one member already.
//
// Members of extra whose name collides with a known property of t are
// dropped: the known property is authoritative, and emitting both would
// produce a duplicate JSON member. This keeps the output valid regardless of
// how a caller populated the Extra map. The decode path never produces such
// a collision, since captureExtra excludes known names.
func spliceExtra(known []byte, extra map[string]json.RawMessage, t reflect.Type, typeName string) ([]byte, error) {
	extra = withoutKnownMembers(extra, t)
	if len(extra) == 0 {
		return known, nil
	}
	extraBytes, err := json.Marshal(extra)
	if err != nil {
		return nil, fmt.Errorf("jscalendar: marshal %s extension members: %w", typeName, err)
	}
	// known ends with '}', extraBytes is '{...}' with at least one member;
	// drop known's closing brace and extraBytes' opening brace, joined by a
	// comma, to merge the two objects into one.
	out := make([]byte, 0, len(known)+len(extraBytes))
	out = append(out, known[:len(known)-1]...)
	out = append(out, ',')
	out = append(out, extraBytes[1:]...)
	return out, nil
}

// withoutKnownMembers returns extra with any member whose key names a known
// JSON property of t removed. It returns the input map unchanged when there
// is no collision (the common case), and otherwise a filtered copy — the
// caller's Extra map is never mutated.
func withoutKnownMembers(extra map[string]json.RawMessage, t reflect.Type) map[string]json.RawMessage {
	known := knownMemberNames(t)
	collides := false
	for k := range extra {
		if _, ok := known[k]; ok {
			collides = true
			break
		}
	}
	if !collides {
		return extra
	}
	filtered := make(map[string]json.RawMessage, len(extra))
	for k, v := range extra {
		if _, ok := known[k]; !ok {
			filtered[k] = v
		}
	}
	return filtered
}

// knownMemberNamesCache memoizes knownMemberNames by struct type. The set is
// derived purely from the type's fields and tags, so it is stable for the
// life of the process.
var knownMemberNamesCache sync.Map // reflect.Type -> map[string]struct{}

// knownMemberNames returns the set of JSON member names that the exported
// fields of struct type t marshal to — the part of each `json:"..."` tag
// before the first comma, or the field name when the tag is absent. Fields
// tagged json:"-" (the Extra field itself) are excluded, as are unexported
// fields. The result is cached and must not be mutated by callers.
func knownMemberNames(t reflect.Type) map[string]struct{} {
	if cached, ok := knownMemberNamesCache.Load(t); ok {
		return cached.(map[string]struct{})
	}
	names := make(map[string]struct{}, t.NumField())
	for i := range t.NumField() {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue // unexported field, never marshaled
		}
		tag, ok := f.Tag.Lookup("json")
		if !ok {
			names[f.Name] = struct{}{}
			continue
		}
		name, _, _ := strings.Cut(tag, ",")
		switch name {
		case "-":
			continue // explicitly not marshaled (the Extra field)
		case "":
			names[f.Name] = struct{}{}
		default:
			names[name] = struct{}{}
		}
	}
	knownMemberNamesCache.Store(t, names)
	return names
}

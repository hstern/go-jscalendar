// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import "encoding/json"

// This file defines the Relation sub-object (RFC 8984, Section 1.4.10), the
// value type of the "relatedTo" property on [Event], [Task], and [Alert]. A
// relatedTo map is keyed by the UID of the related object (a string, not an
// [Id]); each value is a Relation describing how the two objects relate.
//
// Like the other sub-objects, Relation emits its "@type" discriminator first
// and round-trips unknown members losslessly through an Extra field, reusing
// the shared marshalKnown / unmarshalKnown seam (codec.go, extra.go).
// MarshalJSON has a value receiver so a Relation stored as a map value
// marshals through the custom codec. Field declaration order is the wire
// order of the known members; keep Type first.

// Relation is a JSCalendar "Relation" (RFC 8984, Section 1.4.10): the value
// of a relatedTo map entry, describing how the keying object relates to the
// object identified by the map key (a UID string).
//
// The single known member, Relation, is a set of relation-type tokens; every
// member is optional and omitted when zero. Decoding never rejects a
// Relation; such checks belong to the opt-in validation pass.
type Relation struct {
	// Type is the "@type" discriminator and MUST equal "Relation" (Section
	// 1.4.10). First declared field so the codec emits it first; the codec
	// forces the value, so a zero Type still marshals correctly.
	Type string `json:"@type,omitempty"`

	// Relation is the set of relation-type tokens describing how the two
	// objects relate, e.g. {"first": true, "parent": true}; every map value
	// MUST be true (Section 1.4.10). The registered tokens ("first", "next",
	// "child", "parent") are open; an unrecognized token round-trips.
	Relation map[string]bool `json:"relation,omitempty"`

	// Extra holds Relation members with no corresponding known property,
	// preserved verbatim for a lossless, byte-stable round trip (see
	// [Event.Extra] for the rationale). The json:"-" tag reserves the field
	// for the codec; use [DecodeJSON] and [EncodeJSON] for typed access.
	Extra map[string]json.RawMessage `json:"-"`
}

// relationType is the "@type" discriminator value for a Relation (RFC 8984,
// Section 1.4.10), forced onto the wire and the decoded value by the codec.
const relationType = "Relation"

// relationAlias carries the field set and struct tags of [Relation] but none
// of its methods, so encoding/json uses its default struct codec rather than
// recursing into MarshalJSON / UnmarshalJSON.
type relationAlias Relation

// MarshalJSON encodes the Relation with "@type":"Relation" as the first
// member, the known properties in declaration order, and any [Relation.Extra]
// members in sorted key order, byte-stable throughout. A zero or mismatched
// [Relation.Type] is normalized to "Relation".
func (r Relation) MarshalJSON() ([]byte, error) {
	a := relationAlias(r)
	a.Type = relationType
	return marshalKnown(a, relationType, r.Extra)
}

// UnmarshalJSON decodes a JSON object into the Relation, tolerant of member
// order and a missing "@type". [Relation.Type] is set to "Relation" after a
// successful decode regardless of the wire value. Members with no
// corresponding known property are captured into [Relation.Extra].
func (r *Relation) UnmarshalJSON(data []byte) error {
	var a relationAlias
	extra, err := unmarshalKnown(data, &a, relationType, func() { a.Type = relationType })
	if err != nil {
		return err
	}
	*r = Relation(a)
	r.Extra = extra
	return nil
}

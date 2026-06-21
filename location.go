// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import "encoding/json"

// This file defines the two location sub-objects: Location (RFC 8984,
// Section 4.2.5), the value type of the "locations" property, and
// VirtualLocation (Section 4.2.6), the value type of the "virtualLocations"
// property. Both are carried in Id-keyed maps on [Event] and [Task].
//
// Each emits its "@type" discriminator first and round-trips unknown members
// through an Extra field, reusing the shared marshalKnown / unmarshalKnown
// seam exactly as the top-level types do (see codec.go and extra.go). Field
// declaration order is the wire order of the known members; keep Type first.

// Location is a JSCalendar "Location" (RFC 8984, Section 4.2.5): a physical
// location associated with an [Event] or [Task], carried as a value of the
// locations map keyed by a stable [Id].
//
// Every member is optional and omitted from the marshaled object when zero.
// Decoding never rejects a Location; such checks belong to the opt-in
// validation pass.
type Location struct {
	// Type is the "@type" discriminator and MUST equal "Location" (Section
	// 4.2.5). First declared field so the codec emits it first; the codec
	// forces the value, so a zero Type still marshals correctly.
	Type string `json:"@type,omitempty"`

	// Name is the display name of the location (Section 4.2.5).
	Name string `json:"name,omitempty"`
	// Description is a plain-text note describing the location (Section
	// 4.2.5).
	Description string `json:"description,omitempty"`
	// LocationTypes is the set of location-type tokens from the IANA Location
	// Types registry that categorize this location; every map value MUST be
	// true (Section 4.2.5). The keys are open.
	LocationTypes map[string]bool `json:"locationTypes,omitempty"`
	// RelativeTo describes how this location relates to the object's time:
	// "start" or "end" (Section 4.2.5). The value is open; unrecognized
	// values round-trip.
	RelativeTo string `json:"relativeTo,omitempty"`
	// TimeZone is the [TimeZoneId] of this location, used e.g. to render a
	// flight's arrival airport in its local zone (Section 4.2.5).
	TimeZone TimeZoneId `json:"timeZone,omitempty"`
	// Coordinates is a "geo:" URI (RFC 5870) giving the location's
	// geographic position, e.g. "geo:40.7829,-73.9654" (Section 4.2.5).
	Coordinates string `json:"coordinates,omitempty"`
	// Links is the set of links relevant to this location, keyed by [Id]
	// (Section 4.2.5). The Link value type arrives in a later phase; until
	// then the values are kept as [json.RawMessage] so a location carrying a
	// links map round-trips byte-stably.
	//
	// TODO(JSCAL-19): replace json.RawMessage with the typed Link value once
	// the links property and its Link type land.
	Links map[Id]json.RawMessage `json:"links,omitempty"`

	// Extra holds Location members with no corresponding known property,
	// preserved verbatim for a lossless, byte-stable round trip (see
	// [Event.Extra] for the rationale). The json:"-" tag reserves the field
	// for the codec; use [DecodeJSON] and [EncodeJSON] for typed access.
	Extra map[string]json.RawMessage `json:"-"`
}

// locationType is the "@type" discriminator value for a Location (RFC 8984,
// Section 4.2.5), forced onto the wire and the decoded value by the codec.
const locationType = "Location"

// locationAlias carries the field set and struct tags of [Location] but none
// of its methods, so encoding/json uses its default struct codec rather than
// recursing into MarshalJSON / UnmarshalJSON.
type locationAlias Location

// MarshalJSON encodes the Location with "@type":"Location" as the first
// member, the known properties in declaration order, and any
// [Location.Extra] members in sorted key order, byte-stable throughout. A
// zero or mismatched [Location.Type] is normalized to "Location".
func (l Location) MarshalJSON() ([]byte, error) {
	a := locationAlias(l)
	a.Type = locationType
	return marshalKnown(a, locationType, l.Extra)
}

// UnmarshalJSON decodes a JSON object into the Location, tolerant of member
// order and a missing "@type". [Location.Type] is set to "Location" after a
// successful decode regardless of the wire value. Members with no
// corresponding known property are captured into [Location.Extra].
func (l *Location) UnmarshalJSON(data []byte) error {
	var a locationAlias
	extra, err := unmarshalKnown(data, &a, locationType, func() { a.Type = locationType })
	if err != nil {
		return err
	}
	*l = Location(a)
	l.Extra = extra
	return nil
}

// VirtualLocation is a JSCalendar "VirtualLocation" (RFC 8984, Section
// 4.2.6): a virtual location associated with an [Event] or [Task] — a video
// conference, a phone bridge, a chat room — carried as a value of the
// virtualLocations map keyed by a stable [Id].
//
// Every member is optional and omitted when zero. Decoding never rejects a
// VirtualLocation; such checks belong to the opt-in validation pass.
type VirtualLocation struct {
	// Type is the "@type" discriminator and MUST equal "VirtualLocation"
	// (Section 4.2.6). First declared field so the codec emits it first; the
	// codec forces the value, so a zero Type still marshals correctly.
	Type string `json:"@type,omitempty"`

	// Name is the display name of the virtual location (Section 4.2.6).
	Name string `json:"name,omitempty"`
	// Description is a plain-text note describing the virtual location, e.g.
	// joining instructions (Section 4.2.6).
	Description string `json:"description,omitempty"`
	// URI is the resource to use to connect to the virtual location, e.g. a
	// conferencing URL or a "tel:" URI (Section 4.2.6).
	URI string `json:"uri,omitempty"`
	// Features is the set of features the virtual location supports, e.g.
	// "audio", "video", "chat", "screen"; every map value MUST be true
	// (Section 4.2.6). The keys are open.
	Features map[string]bool `json:"features,omitempty"`

	// Extra holds VirtualLocation members with no corresponding known
	// property, preserved verbatim for a lossless, byte-stable round trip
	// (see [Event.Extra] for the rationale). The json:"-" tag reserves the
	// field for the codec; use [DecodeJSON] and [EncodeJSON] for typed
	// access.
	Extra map[string]json.RawMessage `json:"-"`
}

// virtualLocationType is the "@type" discriminator value for a
// VirtualLocation (RFC 8984, Section 4.2.6), forced onto the wire and the
// decoded value by the codec.
const virtualLocationType = "VirtualLocation"

// virtualLocationAlias carries the field set and struct tags of
// [VirtualLocation] but none of its methods, so encoding/json uses its
// default struct codec rather than recursing into MarshalJSON /
// UnmarshalJSON.
type virtualLocationAlias VirtualLocation

// MarshalJSON encodes the VirtualLocation with "@type":"VirtualLocation" as
// the first member, the known properties in declaration order, and any
// [VirtualLocation.Extra] members in sorted key order, byte-stable
// throughout. A zero or mismatched [VirtualLocation.Type] is normalized to
// "VirtualLocation".
func (v VirtualLocation) MarshalJSON() ([]byte, error) {
	a := virtualLocationAlias(v)
	a.Type = virtualLocationType
	return marshalKnown(a, virtualLocationType, v.Extra)
}

// UnmarshalJSON decodes a JSON object into the VirtualLocation, tolerant of
// member order and a missing "@type". [VirtualLocation.Type] is set to
// "VirtualLocation" after a successful decode regardless of the wire value.
// Members with no corresponding known property are captured into
// [VirtualLocation.Extra].
func (v *VirtualLocation) UnmarshalJSON(data []byte) error {
	var a virtualLocationAlias
	extra, err := unmarshalKnown(data, &a, virtualLocationType, func() { a.Type = virtualLocationType })
	if err != nil {
		return err
	}
	*v = VirtualLocation(a)
	v.Extra = extra
	return nil
}

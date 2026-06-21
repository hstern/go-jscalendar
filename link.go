// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import "encoding/json"

// This file defines the Link sub-object (RFC 8984, Section 1.4.11), the value
// type of the "links" property on [Event], [Task], and [Location]. A Link
// associates an external resource — an attachment, an alternate
// representation, a related web page — with the object that carries it.
//
// Like the other sub-objects, Link emits its "@type" discriminator first and
// round-trips unknown members losslessly through an Extra field, reusing the
// shared marshalKnown / unmarshalKnown seam (codec.go, extra.go). MarshalJSON
// has a value receiver so a Link stored as a map value marshals through the
// custom codec, matching the Participant and Location sub-objects. Field
// declaration order is the wire order of the known members; keep Type first.

// Link is a JSCalendar "Link" (RFC 8984, Section 1.4.11): a reference to an
// external resource, carried as a value of a links map keyed by a stable
// [Id].
//
// Only Href is meaningful enough that the spec requires it; every other
// member is optional and omitted from the marshaled object when zero.
// Decoding never rejects a Link for a missing or unrecognized member; such
// checks belong to the opt-in validation pass.
type Link struct {
	// Type is the "@type" discriminator and MUST equal "Link" (Section
	// 1.4.11). First declared field so the codec emits it first; the codec
	// forces the value, so a zero Type still marshals correctly.
	Type string `json:"@type,omitempty"`

	// Href is the URI from which the resource can be fetched (Section
	// 1.4.11). It is the one required member of a Link; the value is a URI
	// reference preserved verbatim.
	Href string `json:"href,omitempty"`
	// CID is the value of a "Content-ID" header (RFC 2392) for an inline
	// resource, allowing Href to be resolved against an embedded MIME part
	// (Section 1.4.11).
	CID string `json:"cid,omitempty"`
	// ContentType is the media type (RFC 6838) of the resource identified by
	// Href, e.g. "image/png" (Section 1.4.11).
	ContentType string `json:"contentType,omitempty"`
	// Size is the size of the resource in octets, as an UnsignedInt (Section
	// 1.4.11). It is an indication, not a guarantee; the value is open.
	Size uint `json:"size,omitempty"`
	// Rel is the relationship between the resource and the linking object,
	// an IANA link-relation type (RFC 8288), e.g. "enclosure" or
	// "alternate" (Section 1.4.11). The value is open.
	Rel string `json:"rel,omitempty"`
	// Display describes the intended use when Rel is "icon", e.g.
	// "badge", "graphic", "fullsize", or "thumbnail" (Section 1.4.11). The
	// value is open.
	Display string `json:"display,omitempty"`
	// Title is a human-readable plain-text description of the resource, for
	// presentation to the user (Section 1.4.11).
	Title string `json:"title,omitempty"`

	// Extra holds Link members with no corresponding known property,
	// preserved verbatim for a lossless, byte-stable round trip (see
	// [Event.Extra] for the rationale). The json:"-" tag reserves the field
	// for the codec; use [DecodeJSON] and [EncodeJSON] for typed access.
	Extra map[string]json.RawMessage `json:"-"`
}

// linkType is the "@type" discriminator value for a Link (RFC 8984, Section
// 1.4.11), forced onto the wire and the decoded value by the codec.
const linkType = "Link"

// linkAlias carries the field set and struct tags of [Link] but none of its
// methods, so encoding/json uses its default struct codec rather than
// recursing into MarshalJSON / UnmarshalJSON.
type linkAlias Link

// MarshalJSON encodes the Link with "@type":"Link" as the first member, the
// known properties in declaration order, and any [Link.Extra] members in
// sorted key order, byte-stable throughout. A zero or mismatched [Link.Type]
// is normalized to "Link".
func (l Link) MarshalJSON() ([]byte, error) {
	a := linkAlias(l)
	a.Type = linkType
	return marshalKnown(a, linkType, l.Extra)
}

// UnmarshalJSON decodes a JSON object into the Link, tolerant of member order
// and a missing "@type". [Link.Type] is set to "Link" after a successful
// decode regardless of the wire value. Members with no corresponding known
// property are captured into [Link.Extra].
func (l *Link) UnmarshalJSON(data []byte) error {
	var a linkAlias
	extra, err := unmarshalKnown(data, &a, linkType, func() { a.Type = linkType })
	if err != nil {
		return err
	}
	*l = Link(a)
	l.Extra = extra
	return nil
}

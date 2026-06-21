// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

// Id is the JSCalendar "Id" value type (RFC 8984, Section 1.4.1).
//
// An Id is a short opaque string of 1 to 255 octets drawn from the
// URL-and-filename-safe Base64 alphabet defined in RFC 4648, Section 5:
// the ASCII letters A–Z and a–z, the digits 0–9, and the two characters
// "-" (hyphen) and "_" (underscore). The padding character "=" is not
// permitted. The RFC recommends, but does not require, that generators use
// a UUID.
//
// Ids identify members within an object — they are the keys of the keyed
// sub-object maps such as participants, locations, links, and alerts — so
// the same Id may appear both as a map key and as a cross-reference value
// elsewhere in the same object.
//
// Following the library's lenient-unmarshal/strict-validate posture, an Id
// decoded from JSON is never rejected for shape; it is an ordinary string on
// the wire. Use [Id.IsValid] to check conformance at a boundary where it
// matters.
//
// The spelling "Id" matches the RFC 8984 type name verbatim. Go's "ID"
// initialism is set aside here for wire/schema fidelity, since this type
// appears pervasively as a map key (for example map[Id]Participant).
//
//nolint:revive // "Id" intentionally tracks the RFC 8984 type name; see above.
type Id string

// maxIDLen is the inclusive upper bound on the length of an Id in octets
// (RFC 8984, Section 1.4.1). Because the permitted alphabet is ASCII, octet
// length equals byte length equals rune count.
const maxIDLen = 255

// IsValid reports whether the Id conforms to RFC 8984, Section 1.4.1: it has
// a length of 1 to 255 octets and every octet is drawn from the
// URL-and-filename-safe Base64 alphabet (A–Z, a–z, 0–9, "-", "_").
//
// IsValid is opt-in: decoding does not call it, so a non-conformant Id can
// still be present on a decoded object and round-trip unchanged.
func (id Id) IsValid() bool {
	if len(id) == 0 || len(id) > maxIDLen {
		return false
	}
	for i := range len(id) {
		if !isIDByte(id[i]) {
			return false
		}
	}
	return true
}

// isIDByte reports whether b is a member of the URL-and-filename-safe Base64
// alphabet permitted in an Id.
func isIDByte(b byte) bool {
	switch {
	case b >= 'A' && b <= 'Z':
		return true
	case b >= 'a' && b <= 'z':
		return true
	case b >= '0' && b <= '9':
		return true
	case b == '-' || b == '_':
		return true
	default:
		return false
	}
}

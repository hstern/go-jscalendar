// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

// TimeZoneId is the JSCalendar "TimeZoneId" value type (RFC 8984,
// Section 1.4.9). It names the time zone of a [LocalDateTime] and takes one
// of two forms:
//
//   - A time zone name from the IANA Time Zone Database (for example
//     "America/New_York" or "Etc/UTC").
//
//   - A custom time zone identifier, which MUST begin with a "/" (solidus)
//     and is resolved against the timeZones map embedded in the JSCalendar
//     object that references it (RFC 8984, Section 4.7.2).
//
// The leading "/" is significant and is part of the identifier: it is the
// signal that the zone is defined locally rather than in the IANA database,
// and the whole string — slash included — is the key used to look the zone
// up in the timeZones map. It MUST be preserved verbatim and MUST NOT be
// interpreted as an IANA name. [TimeZoneId.IsCustom] distinguishes the two
// forms.
//
// Following the library's lenient-unmarshal/strict-validate posture, a
// TimeZoneId decoded from JSON is an ordinary string and is never rejected
// for shape. Use [TimeZoneId.IsValid] for a structural check and
// [TimeZoneId.ResolvesIn] for the custom-zone closure check.
//
// The spelling "TimeZoneId" matches the RFC 8984 type name verbatim. Go's
// "ID" initialism is set aside here for wire/schema fidelity, matching the
// [Id] value type this builds on.
//
//nolint:revive // "TimeZoneId" intentionally tracks the RFC 8984 type name.
type TimeZoneId string

// IsCustom reports whether the identifier names a custom, per-object time
// zone — that is, whether it begins with the "/" prefix defined in RFC 8984,
// Section 1.4.9. A custom zone must be defined in the referencing object's
// timeZones map; an IANA name (IsCustom reports false) is resolved against
// the IANA Time Zone Database instead.
func (tz TimeZoneId) IsCustom() bool {
	return len(tz) > 0 && tz[0] == '/'
}

// IsValid reports whether the TimeZoneId is structurally well-formed.
//
// A custom identifier (one with the leading "/") is valid when it has at
// least one character after the slash. A non-custom identifier is valid when
// it is non-empty; the IANA Time Zone Database is deliberately not consulted,
// both because that would impose a runtime dependency the library does not
// take and because validation is lenient by design — an unrecognized but
// well-shaped name is accepted here and left to the caller's own resolution.
//
// IsValid does not check that a custom zone actually resolves; that closure
// check is [TimeZoneId.ResolvesIn], which needs the object's timeZones map.
func (tz TimeZoneId) IsValid() bool {
	if tz.IsCustom() {
		return len(tz) > 1
	}
	return len(tz) > 0
}

// ResolvesIn reports whether the TimeZoneId resolves to a definition.
//
// A custom identifier (leading "/") resolves only if present reports true for
// it — that is, only if the referencing object's timeZones map contains the
// identifier verbatim (slash included) as a key. A non-custom (IANA)
// identifier needs no embedded definition and always resolves.
//
// The caller supplies present as a closure over its timeZones map so this
// package need not depend on the concrete map type, which lives with the
// object model. A typical call is:
//
//	ok := tz.ResolvesIn(func(k TimeZoneId) bool {
//		_, found := obj.TimeZones[k]
//		return found
//	})
//
// A nil present is treated as an empty timeZones map: a custom identifier
// then resolves to false rather than panicking, since no custom zone can be
// found in the absence of definitions.
func (tz TimeZoneId) ResolvesIn(present func(TimeZoneId) bool) bool {
	if !tz.IsCustom() {
		return true
	}
	if present == nil {
		return false
	}
	return present(tz)
}

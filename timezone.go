// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import "encoding/json"

// This file defines the TimeZone and TimeZoneRule sub-objects (RFC 8984,
// Sections 4.7.2 and 4.7.3), the value types of the embedded "timeZones" map
// on [Event] and [Task]. A TimeZone is a self-contained, per-object time-zone
// definition — the JSCalendar equivalent of an iCalendar VTIMEZONE — used when
// an object references a custom, "/"-prefixed [TimeZoneId] that is not in the
// IANA Time Zone Database. Each TimeZone is keyed in the timeZones map by that
// custom identifier (the leading "/" included) and supplies the standard- and
// daylight-time transition rules that resolve the identifier locally.
//
// A TimeZoneRule expresses one such transition: an offset change effective from
// a [LocalDateTime], optionally repeating via recurrence rules and overrides,
// the way a VTIMEZONE STANDARD or DAYLIGHT component does.
//
// Like the other sub-objects, both types emit their "@type" discriminator
// first and round-trip unknown members losslessly through an Extra field,
// reusing the shared marshalKnown / unmarshalKnown seam (codec.go, extra.go).
// MarshalJSON has a value receiver so a value stored in a map (a TimeZone in
// timeZones, or a TimeZoneRule decoded into a slice) marshals through the
// custom codec. Field declaration order is the wire order of the known
// members; keep Type first.
//
// Closure pin. A custom ("/"-prefixed) [TimeZoneId] referenced anywhere in an
// object — in the object's own "timeZone", in a location's "timeZone", and so
// on — MUST resolve to a TimeZone in that object's timeZones map (RFC 8984,
// Section 4.7.1). This package does not enforce that closure on decode, in
// keeping with the lenient-unmarshal posture; the check is the validation
// phase's job, via [TimeZoneId.ResolvesIn] over the object's TimeZones map.

// TimeZone is a JSCalendar "TimeZone" (RFC 8984, Section 4.7.2): a custom,
// per-object time-zone definition resolved by a "/"-prefixed [TimeZoneId]. It
// mirrors an iCalendar VTIMEZONE: a tzId, optional metadata, and the standard-
// and daylight-time transition rules ([TimeZoneRule]) that define the zone's
// offsets over time.
//
// Every member but Type is optional and omitted from the marshaled object when
// zero. Decoding never rejects a TimeZone; structural checks — including the
// closure that a referenced custom zone is actually defined — belong to the
// opt-in validation pass.
type TimeZone struct {
	// Type is the "@type" discriminator and MUST equal "TimeZone" (Section
	// 4.7.2). First declared field so the codec emits it first; the codec
	// forces the value, so a zero Type still marshals correctly.
	Type string `json:"@type,omitempty"`

	// TzID is the time-zone identifier this definition is known by, e.g.
	// "/Example/Custom" (Section 4.7.2). It is the IANA-style name the zone
	// would carry if it were registered; the leading "/" of the custom
	// [TimeZoneId] that keys this TimeZone in the timeZones map is part of the
	// map key, not necessarily of TzID.
	TzID string `json:"tzId,omitempty"`
	// Updated is the UTC date-time this definition was last modified (Section
	// 4.7.2), used to decide whether a cached copy of the zone is stale.
	Updated *UTCDateTime `json:"updated,omitempty"`
	// URL is a location from which this time-zone definition may be retrieved
	// (Section 4.7.2).
	URL string `json:"url,omitempty"`
	// ValidUntil is the UTC date-time after which this definition's rules are
	// no longer guaranteed to be correct (Section 4.7.2); a consumer should
	// refresh the zone past this instant.
	ValidUntil *UTCDateTime `json:"validUntil,omitempty"`
	// Aliases is the set of additional identifiers this zone is also known by;
	// every value MUST be true (Section 4.7.2). An unrecognized alias
	// round-trips.
	Aliases map[string]bool `json:"aliases,omitempty"`
	// Standard is the list of rules describing the zone's standard-time
	// (non-daylight) offsets and their transitions (Section 4.7.2), analogous
	// to the STANDARD sub-components of a VTIMEZONE.
	Standard []TimeZoneRule `json:"standard,omitempty"`
	// Daylight is the list of rules describing the zone's daylight-saving-time
	// offsets and their transitions (Section 4.7.2), analogous to the DAYLIGHT
	// sub-components of a VTIMEZONE.
	Daylight []TimeZoneRule `json:"daylight,omitempty"`

	// Extra holds TimeZone members with no corresponding known property,
	// preserved verbatim for a lossless, byte-stable round trip (see
	// [Event.Extra] for the rationale). The json:"-" tag reserves the field
	// for the codec; use [DecodeJSON] and [EncodeJSON] for typed access.
	Extra map[string]json.RawMessage `json:"-"`
}

// TimeZoneRule is a JSCalendar "TimeZoneRule" (RFC 8984, Section 4.7.3): one
// standard- or daylight-time transition of a [TimeZone]. It states the offset
// in effect from a [LocalDateTime] onward and, optionally, how that transition
// repeats — the JSCalendar form of a VTIMEZONE STANDARD or DAYLIGHT component.
//
// Start, OffsetFrom, and OffsetTo are the substantive members of a rule; every
// member but Type is optional on the wire and omitted when zero. The recurrence
// members reuse [RecurrenceRule] and the PatchObject override map, exactly as
// they are used on [Event] and [Task], so a custom zone expresses its repeating
// DST transitions with the same machinery as an event's own recurrence.
// Decoding never rejects a rule; such checks belong to the validation pass.
type TimeZoneRule struct {
	// Type is the "@type" discriminator and MUST equal "TimeZoneRule" (Section
	// 4.7.3). First declared field so the codec emits it first; the codec
	// forces the value, so a zero Type still marshals correctly.
	Type string `json:"@type,omitempty"`

	// Start is the [LocalDateTime] from which this rule's offset takes effect,
	// interpreted in the OffsetFrom offset (Section 4.7.3). It is the one
	// member the spec makes mandatory; it is a pointer so an absent Start
	// round-trips as absent rather than as the zero LocalDateTime, keeping the
	// codec lenient and byte-stable.
	Start *LocalDateTime `json:"start,omitempty"`
	// OffsetFrom is the UTC offset in effect immediately before this
	// transition, as a signed "+HH:MM" / "-HH:MM" UTCOffset (Section 4.7.3).
	OffsetFrom string `json:"offsetFrom,omitempty"`
	// OffsetTo is the UTC offset in effect from Start onward, as a signed
	// UTCOffset (Section 4.7.3).
	OffsetTo string `json:"offsetTo,omitempty"`
	// RecurrenceRules are the rules by which this transition repeats (Section
	// 4.7.3), reusing the event-level [RecurrenceRule] type. Each rule is
	// evaluated against Start, the way an event's recurrence is evaluated
	// against its own start.
	RecurrenceRules []RecurrenceRule `json:"recurrenceRules,omitempty"`
	// RecurrenceOverrides maps a recurrence-id [LocalDateTime] (as a string)
	// to a [PatchObject] that adjusts the rule for that one occurrence
	// (Section 4.7.3), reusing the same override mechanism as
	// [Event.RecurrenceOverrides].
	RecurrenceOverrides map[string]PatchObject `json:"recurrenceOverrides,omitempty"`
	// Names is the set of human-readable names this offset is presented under,
	// e.g. {"EST": true}; every value MUST be true (Section 4.7.3). An
	// unrecognized name round-trips.
	Names map[string]bool `json:"names,omitempty"`
	// Comments are free-text notes accompanying the rule (Section 4.7.3),
	// mirroring a VTIMEZONE component's COMMENT properties.
	Comments []string `json:"comments,omitempty"`

	// Extra holds TimeZoneRule members with no corresponding known property,
	// preserved verbatim for a lossless, byte-stable round trip (see
	// [Event.Extra] for the rationale). The json:"-" tag reserves the field
	// for the codec; use [DecodeJSON] and [EncodeJSON] for typed access.
	Extra map[string]json.RawMessage `json:"-"`
}

// timeZoneType and timeZoneRuleType are the "@type" discriminator values for
// [TimeZone] and [TimeZoneRule] (RFC 8984, Sections 4.7.2 and 4.7.3), forced
// onto the wire and the decoded value by the codec.
const (
	timeZoneType     = "TimeZone"
	timeZoneRuleType = "TimeZoneRule"
)

// timeZoneAlias and timeZoneRuleAlias carry the field set and struct tags of
// their named types but none of their methods, so encoding/json uses its
// default struct codec rather than recursing into MarshalJSON / UnmarshalJSON.
type (
	timeZoneAlias     TimeZone
	timeZoneRuleAlias TimeZoneRule
)

// MarshalJSON encodes the TimeZone with "@type":"TimeZone" as the first
// member, the known properties in declaration order, and any [TimeZone.Extra]
// members in sorted key order, byte-stable throughout. A zero or mismatched
// [TimeZone.Type] is normalized to "TimeZone".
func (z TimeZone) MarshalJSON() ([]byte, error) {
	a := timeZoneAlias(z)
	a.Type = timeZoneType
	return marshalKnown(a, timeZoneType, z.Extra)
}

// UnmarshalJSON decodes a JSON object into the TimeZone, tolerant of member
// order and a missing "@type". [TimeZone.Type] is set to "TimeZone" after a
// successful decode regardless of the wire value. Members with no corresponding
// known property are captured into [TimeZone.Extra].
func (z *TimeZone) UnmarshalJSON(data []byte) error {
	var a timeZoneAlias
	extra, err := unmarshalKnown(data, &a, timeZoneType, func() { a.Type = timeZoneType })
	if err != nil {
		return err
	}
	*z = TimeZone(a)
	z.Extra = extra
	return nil
}

// MarshalJSON encodes the TimeZoneRule with "@type":"TimeZoneRule" as the first
// member, the known properties in declaration order, and any
// [TimeZoneRule.Extra] members in sorted key order, byte-stable throughout. A
// zero or mismatched [TimeZoneRule.Type] is normalized to "TimeZoneRule".
func (r TimeZoneRule) MarshalJSON() ([]byte, error) {
	a := timeZoneRuleAlias(r)
	a.Type = timeZoneRuleType
	return marshalKnown(a, timeZoneRuleType, r.Extra)
}

// UnmarshalJSON decodes a JSON object into the TimeZoneRule, tolerant of member
// order and a missing "@type". [TimeZoneRule.Type] is set to "TimeZoneRule"
// after a successful decode regardless of the wire value. Members with no
// corresponding known property are captured into [TimeZoneRule.Extra].
func (r *TimeZoneRule) UnmarshalJSON(data []byte) error {
	var a timeZoneRuleAlias
	extra, err := unmarshalKnown(data, &a, timeZoneRuleType, func() { a.Type = timeZoneRuleType })
	if err != nil {
		return err
	}
	*r = TimeZoneRule(a)
	r.Extra = extra
	return nil
}

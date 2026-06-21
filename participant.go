// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import "encoding/json"

// This file defines the Participant sub-object (RFC 8984, Section 4.4.6),
// the value type of the Event and Task "participants" property. A
// Participant describes a calendar user involved in the object — an
// attendee, the owner, a chair, an informational contact — together with
// their scheduling and reply state.
//
// Like the three top-level object types, Participant emits its "@type"
// discriminator first and round-trips unknown members losslessly through an
// Extra field. The codec reuses the shared marshalKnown / unmarshalKnown
// seam (codec.go, extra.go): MarshalJSON encodes through a method-stripped
// participantAlias with the discriminator forced on, then splices the Extra
// members in sorted key order; UnmarshalJSON decodes through the alias and
// captures any member it did not consume into Extra. Field declaration
// order below is the wire order of the known members, so reordering fields
// changes the marshaled bytes; keep Type first.
//
// Enum-valued members — Kind, ParticipationStatus, the keys of Roles,
// ScheduleAgent, ScheduleStatus — are modeled as open strings rather than
// closed Go enums. RFC 8984 registers initial values for each but leaves the
// registries open (Section 7), so an unrecognized value is data to preserve,
// not an error to reject; a closed enum would force such a value through the
// Extra escape hatch or drop it. Validation of the registered set, where a
// consumer wants it, is the opt-in validation phase's job.

// Participant is a JSCalendar "Participant" (RFC 8984, Section 4.4.6): a
// calendar user involved in an [Event] or [Task], carried as a value of the
// participants map keyed by a stable [Id].
//
// Every member below is optional and omitted from the marshaled object when
// it holds its zero value, following the library's lenient-unmarshal,
// compact-marshal posture. Decoding never rejects a Participant for a
// missing or unrecognized member; such checks belong to the opt-in
// validation pass.
type Participant struct {
	// Type is the "@type" discriminator and MUST equal "Participant"
	// (Section 4.4.6). It is the first declared field so the codec emits it
	// first; the codec forces the value, so a zero Type still marshals
	// correctly.
	Type string `json:"@type,omitempty"`

	// Name is the display name of the participant, e.g. their full name
	// (Section 4.4.6).
	Name string `json:"name,omitempty"`
	// Email is the participant's email address (Section 4.4.6). It is
	// informational; the addresses to actually send scheduling messages to
	// are in SendTo.
	Email string `json:"email,omitempty"`
	// Description is a plain-text note about the participant, e.g. their
	// role in a meeting in human terms (Section 4.4.6).
	Description string `json:"description,omitempty"`
	// SendTo maps a method to the URI scheduling messages for this
	// participant should be sent to, e.g. {"imip": "mailto:..."} (Section
	// 4.4.6). The method key is open; "imip" and "other" are registered.
	SendTo map[string]string `json:"sendTo,omitempty"`
	// Kind is the kind of entity the participant represents, e.g.
	// "individual", "group", "resource", or "location" (Section 4.4.6). The
	// value is open: unrecognized kinds round-trip.
	Kind string `json:"kind,omitempty"`
	// Roles is the set of roles the participant has, e.g.
	// {"attendee": true, "chair": true}; every map value MUST be true
	// (Section 4.4.6). The role keys ("owner", "attendee", "optional",
	// "informational", "chair", "contact") are open.
	Roles map[string]bool `json:"roles,omitempty"`
	// ParticipationStatus is the participant's response to the invitation,
	// e.g. "needs-action", "accepted", "declined", "tentative", or
	// "delegated" (Section 4.4.6); default "needs-action". The value is open.
	ParticipationStatus string `json:"participationStatus,omitempty"`
	// ParticipationComment is a note the participant sent with their
	// ParticipationStatus, e.g. a reason for declining (Section 4.4.6).
	ParticipationComment string `json:"participationComment,omitempty"`
	// ExpectReply indicates whether the organizer expects this participant
	// to reply to the invitation (Section 4.4.6); default false.
	ExpectReply bool `json:"expectReply,omitempty"`
	// ScheduleAgent names which entity is expected to deliver scheduling
	// messages for this participant: "server", "client", or "none" (Section
	// 4.4.6); default "server". The value is open.
	ScheduleAgent string `json:"scheduleAgent,omitempty"`
	// ScheduleForceSend indicates the organizer's client should send a
	// scheduling message to this participant even if it judges none is
	// required (Section 4.4.6); default false.
	ScheduleForceSend bool `json:"scheduleForceSend,omitempty"`
	// ScheduleSequence is the sequence number of the last scheduling message
	// sent to or received from this participant (Section 4.4.6); default 0.
	ScheduleSequence uint `json:"scheduleSequence,omitempty"`
	// ScheduleStatus holds the iTIP REQUEST-STATUS codes returned for the
	// most recent scheduling message sent to this participant (Section
	// 4.4.6). The codes are open strings.
	ScheduleStatus []string `json:"scheduleStatus,omitempty"`
	// ScheduleUpdated is the UTC date-time at which the participant's
	// scheduling state was last updated by a scheduling message (Section
	// 4.4.6).
	ScheduleUpdated *UTCDateTime `json:"scheduleUpdated,omitempty"`
	// InvitedBy is the [Id] of the participant that invited this one, keyed
	// into the same participants map (Section 4.4.6).
	InvitedBy Id `json:"invitedBy,omitempty"`
	// DelegatedTo is the set of participant [Id]s this participant has
	// delegated their attendance to; every map value MUST be true (Section
	// 4.4.6).
	DelegatedTo map[Id]bool `json:"delegatedTo,omitempty"`
	// DelegatedFrom is the set of participant [Id]s that delegated their
	// attendance to this participant; every map value MUST be true (Section
	// 4.4.6).
	DelegatedFrom map[Id]bool `json:"delegatedFrom,omitempty"`
	// MemberOf is the set of group-participant [Id]s this participant is a
	// member of; every map value MUST be true (Section 4.4.6).
	MemberOf map[Id]bool `json:"memberOf,omitempty"`
	// Links is the set of [Id]s, keyed into the object's links property, of
	// links relevant to this participant; every map value MUST be true
	// (Section 4.4.6). The values reference [Link] entries by their map [Id];
	// the set itself carries no Link bodies, only the cross-references.
	Links map[Id]bool `json:"linkIds,omitempty"`
	// Language is the BCP 47 language tag for the participant's preferred
	// language for communication (Section 4.4.6).
	Language string `json:"language,omitempty"`

	// Extra holds Participant members with no corresponding known property —
	// vendor extensions and properties from future spec revisions, preserved
	// verbatim for a lossless, byte-stable round trip (see [Event.Extra] for
	// the full rationale). The json:"-" tag reserves the field for the
	// codec; use [DecodeJSON] and [EncodeJSON] for typed access.
	Extra map[string]json.RawMessage `json:"-"`
}

// participantType is the "@type" discriminator value for a Participant (RFC
// 8984, Section 4.4.6), forced onto the wire and the decoded value by the
// codec.
const participantType = "Participant"

// participantAlias carries the field set and struct tags of [Participant]
// but none of its methods, so encoding/json applies its default struct codec
// instead of recursing into MarshalJSON / UnmarshalJSON. It is the mechanism
// by which the codec delegates per-field encoding while retaining control of
// the "@type" member and the Extra splice.
type participantAlias Participant

// MarshalJSON encodes the Participant with "@type":"Participant" as the first
// member, the remaining known properties in declaration order, and any
// [Participant.Extra] members in sorted key order, yielding byte-stable
// output. A zero or mismatched [Participant.Type] is normalized to
// "Participant".
func (p Participant) MarshalJSON() ([]byte, error) {
	a := participantAlias(p)
	a.Type = participantType
	return marshalKnown(a, participantType, p.Extra)
}

// UnmarshalJSON decodes a JSON object into the Participant. It is tolerant of
// member order and of a missing "@type"; strict checks belong to the
// validation phase. [Participant.Type] is set to "Participant" after a
// successful decode regardless of the wire value. Members with no
// corresponding known property are captured into [Participant.Extra] for
// lossless round-tripping.
func (p *Participant) UnmarshalJSON(data []byte) error {
	var a participantAlias
	extra, err := unmarshalKnown(data, &a, participantType, func() { a.Type = participantType })
	if err != nil {
		return err
	}
	*p = Participant(a)
	p.Extra = extra
	return nil
}

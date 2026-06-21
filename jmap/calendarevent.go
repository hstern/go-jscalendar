// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

// Package jmap maps the JMAP Calendars "CalendarEvent" object to and from the
// core [jscalendar.Event] type.
//
// A JMAP CalendarEvent is, on the wire, a JSCalendar Event (RFC 8984) carrying
// a small set of additional members that the JMAP for Calendars specification
// layers on top of the JSCalendar object — calendarIds, isDraft, utcStart, and
// the rest. This package models exactly that object: a [CalendarEvent] embeds a
// [jscalendar.Event] and adds the JMAP-specific properties as known members,
// with a JSON codec that round-trips the whole object byte-stably and preserves
// unknown members through the embedded Event's [jscalendar.Event] Extra map.
//
// # Scope: object mapping only
//
// This package covers the CalendarEvent *object* and its mapping to and from a
// JSCalendar Event. It deliberately does NOT implement the JMAP transport
// layer: the Calendar/* and CalendarEvent/* method request/response framing —
// /get, /set, /query, /changes — the account/session model, capability
// negotiation, synthetic-id recurrence expansion at query time, and scheduling
// (iTIP) message exchange are all the consumer's concern. A JMAP server or
// client built on this library frames those methods itself and uses the types
// here for the event payloads they carry.
//
// The boundary is the object on the wire, not the protocol around it: given the
// JSON bytes of a CalendarEvent (the value of a CalendarEvent/get "list"
// element, or the argument to a CalendarEvent/set "create"), this package
// decodes it to a [CalendarEvent] and re-encodes it byte-stably; conversely it
// turns a [jscalendar.Event] into the CalendarEvent shape a JMAP method would
// return.
//
// # Standard library only
//
// Like the core jscalendar package, this package depends on nothing outside the
// standard library and the core package itself; it does not pull in the iCal or
// recurrence-expansion sub-packages or any of their external dependencies.
package jmap

import (
	"encoding/json"

	"github.com/hstern/go-jscalendar"
)

// CalendarEvent is the JMAP Calendars "CalendarEvent" object
// (draft-ietf-jmap-calendars, Section 5): a JSCalendar [jscalendar.Event]
// (RFC 8984) plus the additional properties JMAP defines for an event stored in
// a calendar.
//
// # Composition
//
// CalendarEvent embeds a *[jscalendar.Event] rather than copying its fields.
// The embedding is the literal reading of the spec — "It is a JSCalendar Event
// object [...] with the following additional properties" — and it means every
// JSCalendar property (Start, Duration, Participants, RecurrenceRules, the open
// Extra map, the whole surface) is reached through the embedded value and stays
// in exactly one place. The pointer (rather than a value embed) lets a zero
// CalendarEvent distinguish "no underlying event" (nil) from "an event with all
// zero fields", and lets [FromEvent] alias the caller's Event without a deep
// copy. A nil embedded Event marshals as the JMAP members alone over an
// otherwise empty object; the codec tolerates it.
//
// # JMAP additions
//
// The added fields below are the members JMAP layers onto the JSCalendar Event.
// Several are server-set or server-computed and read-only to a client; that is
// a transport-layer contract (which the /set method enforces) and not something
// this object mapping enforces — the fields round-trip whatever the wire
// carries, consistent with the library's lenient-unmarshal posture. The godoc
// on each field records its spec status so a consumer building the transport
// layer knows which it must reject or ignore on write.
//
// # Restricted JSCalendar properties
//
// JMAP forbids one JSCalendar property at the CalendarEvent top level and gives
// special meaning to two others; this package does not silently drop any of
// them. See the package-level discussion and [CalendarEvent.MarshalJSON] for
// how method, privacy, and prodId are handled:
//
//   - method (RFC 8984, Section 4.1.8): a CalendarEvent MUST NOT carry it
//     (draft-ietf-jmap-calendars, Section 5) — it is meaningful only on an iTIP
//     scheduling message, not on a stored event. The field exists on the
//     embedded Event and is preserved on round-trip; [CalendarEvent.Restricted]
//     reports its presence so a transport layer can reject it. It is not erased
//     here, because the object mapping is lossless by contract.
//   - privacy (RFC 8984, Section 4.4.3): carried verbatim. JMAP gives "private"
//     and "secret" server-side access-control meaning (Section 5.7); enforcing
//     that filtering is the server's job, not the object mapping's.
//   - prodId (RFC 8984, Section 4.1.4): carried verbatim. JMAP servers set it
//     themselves; the mapping neither generates nor strips it.
type CalendarEvent struct {
	// Event is the underlying JSCalendar Event (RFC 8984). It is embedded, so
	// every JSCalendar property is promoted onto CalendarEvent. It may be nil,
	// in which case only the JMAP members below are present on the wire.
	*jscalendar.Event

	// ID is the server-set, immutable identifier that uniquely identifies a
	// JSCalendar Event with a particular uid and recurrenceId within an account
	// (draft-ietf-jmap-calendars, Section 5). It is distinct from the
	// JSCalendar uid (which is shared across an account's copies of the same
	// logical event); the JMAP id is account-scoped. A client MUST NOT set it
	// on create; this mapping preserves whatever the wire carries.
	ID jscalendar.Id `json:"id,omitempty"`

	// BaseEventID is set only when ID is a synthetic id the server generated to
	// represent one instance of a recurring event; it gives the id of the
	// stored CalendarEvent the instance was expanded from
	// (draft-ietf-jmap-calendars, Section 5). Server-set and immutable. A
	// pointer so a JSON null (the spec's "this is not a synthetic instance"
	// value) is distinguishable from an absent member.
	BaseEventID *jscalendar.Id `json:"baseEventId,omitempty"`

	// CalendarIDs is the set of Calendar ids this event belongs to
	// (draft-ietf-jmap-calendars, Section 5), modeled as the spec's
	// Id[Boolean]: an object whose every value MUST be true. An event MUST
	// belong to at least one Calendar while it exists; that invariant is a
	// transport-layer (CalendarEvent/set) check, not enforced by this mapping.
	CalendarIDs map[jscalendar.Id]bool `json:"calendarIds,omitempty"`

	// IsDraft reports whether the event is a draft: the server sends no
	// scheduling messages and no alert push notifications for a draft
	// (draft-ietf-jmap-calendars, Section 5). It may be set true only on
	// create and, once false, cannot return to true; it MUST NOT appear in
	// recurrenceOverrides. Those are write-path rules for the transport layer;
	// the object mapping round-trips the value as given.
	IsDraft bool `json:"isDraft,omitempty"`

	// IsOrigin reports whether this account is the authoritative source that
	// controls scheduling for the event (draft-ietf-jmap-calendars, Section 5).
	// Server-set. A pointer so an explicit false is distinguishable from an
	// absent member, since the property is server-set rather than defaulted on
	// the client.
	IsOrigin *bool `json:"isOrigin,omitempty"`

	// UTCStart is the event start expressed in UTC for simple clients without
	// time-zone support (draft-ietf-jmap-calendars, Section 5). It is computed
	// by the server at fetch time from start/timeZone/duration, is NOT returned
	// by default (a client requests it explicitly), and may differ between
	// fetches if time-zone data changed. Effectively read-only; this mapping
	// carries it when present and never synthesizes it (computing it correctly
	// needs the server's current time-zone database). A pointer so its absence —
	// the common case — is distinct from a zero instant.
	UTCStart *jscalendar.UTCDateTime `json:"utcStart,omitempty"`

	// UTCEnd is the event end in UTC, computed by the server from
	// start/timeZone/duration at fetch time (draft-ietf-jmap-calendars, Section
	// 5). Like UTCStart it is not returned by default, is effectively
	// read-only, and is carried verbatim rather than computed here. A pointer
	// for the same absent-versus-zero reason as UTCStart.
	UTCEnd *jscalendar.UTCDateTime `json:"utcEnd,omitempty"`

	// MayInviteSelf, if true, lets anyone add themselves to the event as an
	// "attendee" participant (draft-ietf-jmap-calendars, Section 5.1.1). It is
	// one of the common-use JSCalendar properties JMAP defines; it may be set
	// only on the base object and MUST NOT be altered in recurrenceOverrides.
	MayInviteSelf bool `json:"mayInviteSelf,omitempty"`

	// MayInviteOthers, if true, lets any current "attendee" participant add
	// further "attendee" participants (draft-ietf-jmap-calendars, Section
	// 5.1.2). Base-object only, as for MayInviteSelf.
	MayInviteOthers bool `json:"mayInviteOthers,omitempty"`

	// HideAttendees, if true, restricts the visible participant set to event
	// owners (and the fetching user's own identities) when a non-owner fetches
	// the event (draft-ietf-jmap-calendars, Section 5.1.3). Base-object only.
	// The hiding itself is performed server-side; this mapping only round-trips
	// the flag.
	HideAttendees bool `json:"hideAttendees,omitempty"`
}

// FromEvent wraps a JSCalendar [jscalendar.Event] as a [CalendarEvent] with no
// JMAP-specific properties set, the conversion a JMAP layer performs when it
// has a bare JSCalendar Event (for example one produced by the iCal adapter)
// and needs to present it as a CalendarEvent.
//
// The returned CalendarEvent aliases ev — it is not deep-copied — so the
// embedded Event and the caller's value share storage; mutating one is visible
// through the other. Pass a nil ev to obtain a CalendarEvent with a nil
// embedded Event. The JMAP fields (id, calendarIds, and so on) are left at
// their zero values for the caller to populate; in particular calendarIds is
// nil, which a transport layer must fill before a CalendarEvent/set create,
// since the spec requires membership in at least one Calendar.
func FromEvent(ev *jscalendar.Event) *CalendarEvent {
	return &CalendarEvent{Event: ev}
}

// ToEvent returns the underlying JSCalendar [jscalendar.Event], discarding the
// JMAP-specific properties — the conversion a consumer performs to hand the
// plain JSCalendar object to code that speaks RFC 8984 rather than JMAP (the
// iCal adapter, a validator, a recurrence expander).
//
// The returned pointer is the embedded Event itself, not a copy, so it shares
// storage with the CalendarEvent; it is nil when the CalendarEvent has no
// underlying event. The JMAP members are intentionally dropped: they have no
// representation in a bare JSCalendar Event. FromEvent followed by ToEvent
// therefore returns the original Event pointer unchanged, and ToEvent followed
// by FromEvent reconstructs an equivalent CalendarEvent with empty JMAP fields.
func (ce *CalendarEvent) ToEvent() *jscalendar.Event {
	if ce == nil {
		return nil
	}
	return ce.Event
}

// Restricted reports the JSCalendar properties present on the embedded Event
// that JMAP forbids or restricts at the CalendarEvent top level, for a
// transport layer that wants to reject or sanitize an event before storing it.
//
// Today it reports exactly one condition: a non-empty method property, which a
// CalendarEvent MUST NOT carry (draft-ietf-jmap-calendars, Section 5), since
// method is meaningful only on an iTIP scheduling message. The returned slice
// names the offending JSON members ("method"); it is empty when the event
// carries none of them. This is advisory — the object mapping itself never
// drops a property, so a caller that needs the restriction enforced inspects
// this result and acts on it.
func (ce *CalendarEvent) Restricted() []string {
	if ce == nil || ce.Event == nil || ce.Method == "" {
		return nil
	}
	return []string{"method"}
}

// jmapMemberNames is the set of JSON member names this package owns — the JMAP
// additions to the JSCalendar Event object. The codec uses it to split a
// CalendarEvent's wire object into the JMAP members (decoded here) and the
// JSCalendar members (handed to the embedded Event's decoder), and to splice
// the two halves back together on encode. Keep it in sync with the json tags on
// CalendarEvent's own fields above.
var jmapMemberNames = map[string]struct{}{
	"id":              {},
	"baseEventId":     {},
	"calendarIds":     {},
	"isDraft":         {},
	"isOrigin":        {},
	"utcStart":        {},
	"utcEnd":          {},
	"mayInviteSelf":   {},
	"mayInviteOthers": {},
	"hideAttendees":   {},
}

// jmapOnly is the method-stripped twin of CalendarEvent carrying only the JMAP
// member fields (no embedded Event), so encoding/json applies its default
// struct codec to it without recursing into CalendarEvent.MarshalJSON. The
// field set and tags mirror the JMAP fields on CalendarEvent exactly; the codec
// converts between the two by assigning field-for-field.
type jmapOnly struct {
	ID              jscalendar.Id           `json:"id,omitempty"`
	BaseEventID     *jscalendar.Id          `json:"baseEventId,omitempty"`
	CalendarIDs     map[jscalendar.Id]bool  `json:"calendarIds,omitempty"`
	IsDraft         bool                    `json:"isDraft,omitempty"`
	IsOrigin        *bool                   `json:"isOrigin,omitempty"`
	UTCStart        *jscalendar.UTCDateTime `json:"utcStart,omitempty"`
	UTCEnd          *jscalendar.UTCDateTime `json:"utcEnd,omitempty"`
	MayInviteSelf   bool                    `json:"mayInviteSelf,omitempty"`
	MayInviteOthers bool                    `json:"mayInviteOthers,omitempty"`
	HideAttendees   bool                    `json:"hideAttendees,omitempty"`
}

// jmapView projects the JMAP fields of a CalendarEvent into a jmapOnly.
func (ce *CalendarEvent) jmapView() jmapOnly {
	return jmapOnly{
		ID:              ce.ID,
		BaseEventID:     ce.BaseEventID,
		CalendarIDs:     ce.CalendarIDs,
		IsDraft:         ce.IsDraft,
		IsOrigin:        ce.IsOrigin,
		UTCStart:        ce.UTCStart,
		UTCEnd:          ce.UTCEnd,
		MayInviteSelf:   ce.MayInviteSelf,
		MayInviteOthers: ce.MayInviteOthers,
		HideAttendees:   ce.HideAttendees,
	}
}

// setJMAP copies the decoded JMAP members from v onto the CalendarEvent.
func (ce *CalendarEvent) setJMAP(v jmapOnly) {
	ce.ID = v.ID
	ce.BaseEventID = v.BaseEventID
	ce.CalendarIDs = v.CalendarIDs
	ce.IsDraft = v.IsDraft
	ce.IsOrigin = v.IsOrigin
	ce.UTCStart = v.UTCStart
	ce.UTCEnd = v.UTCEnd
	ce.MayInviteSelf = v.MayInviteSelf
	ce.MayInviteOthers = v.MayInviteOthers
	ce.HideAttendees = v.HideAttendees
}

// ensure the value satisfies the encoding/json interfaces.
var (
	_ json.Marshaler   = CalendarEvent{}
	_ json.Unmarshaler = (*CalendarEvent)(nil)
)

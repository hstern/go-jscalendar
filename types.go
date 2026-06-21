// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package jscalendar

import "encoding/json"

// This file defines the three top-level JSCalendar object types — [Event],
// [Task], and [Group] (RFC 8984, Section 2) — together with the common
// metadata, descriptive, scheduling, and recurrence properties they share
// (Sections 4.1–4.5, 4.7) and the per-type scheduling properties (Section
// 5).
//
// Scope note: only the properties whose value types already exist in the
// package are modeled here. The properties whose value types arrive in a
// later phase — participants, locations, virtualLocations, links,
// relatedTo, alerts, localizations, and the embedded timeZones map — are
// deliberately omitted for now; see the TODO markers below. Each will be
// added alongside its value type so the struct never carries a field whose
// type does not yet compile.
//
// Marshaling note: the JSON codec for these three types lives in codec.go.
// It emits the "@type" member first and always present (RFC 8984, Section
// 1.4.1), forcing the discriminator to the correct value even when the
// Type field is left zero, and decodes tolerantly (member order is
// ignored, a missing "@type" is accepted — strict checks are the
// validation phase's job). The codec relies on the field declaration order
// below for byte-stable output, so reordering fields changes the wire
// order; keep "@type" first. An open-extension Extra field for lossless
// round-trip of unknown members lands in a later phase; codec.go documents
// the seam where it slots in.

// Event is a JSCalendar "Event" (RFC 8984, Section 2.1): a scheduled item
// occupying a region of time, anchored by a [Event.Start] and a
// [Event.Duration].
//
// Of the members below only Type and UID are required by the spec; every
// other property is optional and omitted from the marshaled object when it
// holds its zero value. Following the library's lenient-unmarshal posture,
// decoding never rejects an Event for a missing or out-of-range property —
// such checks belong to a later opt-in validation pass.
type Event struct {
	// Type is the "@type" discriminator and MUST equal "Event" (Section
	// 4.1.1). It is the first declared field so the codec emits it first;
	// the codec also forces the value to "Event", so a zero Type still
	// marshals correctly.
	Type string `json:"@type,omitempty"`

	// --- Metadata properties (Section 4.1) ---

	// UID is the globally unique, lifetime-stable identifier of the object
	// (Section 4.1.2). It is required on a top-level object.
	UID string `json:"uid,omitempty"`
	// ProdID identifies the product that created the object (Section
	// 4.1.4).
	ProdID string `json:"prodId,omitempty"`
	// Created is the UTC date-time at which the object was created
	// (Section 4.1.5).
	Created *UTCDateTime `json:"created,omitempty"`
	// Updated is the UTC date-time at which the object was last modified
	// (Section 4.1.6).
	Updated *UTCDateTime `json:"updated,omitempty"`
	// Sequence is the revision number, incremented on every change except
	// participant-only changes (Section 4.1.7); default 0.
	Sequence uint `json:"sequence,omitempty"`
	// Method is the iTIP scheduling method of the object (Section 4.1.8).
	Method string `json:"method,omitempty"`

	// --- Descriptive properties (Section 4.2) ---

	// Title is the short summary of the object (Section 4.2.1).
	Title string `json:"title,omitempty"`
	// Description is the longer free-text description (Section 4.2.2).
	Description string `json:"description,omitempty"`
	// DescriptionContentType is the media type of Description (Section
	// 4.2.3); default "text/plain".
	DescriptionContentType string `json:"descriptionContentType,omitempty"`
	// ShowWithoutTime indicates the time of day is not significant for
	// display, e.g. an all-day event (Section 4.2.4); default false.
	ShowWithoutTime bool `json:"showWithoutTime,omitempty"`
	// Keywords is a set of free-form keywords; every map value MUST be
	// true (Section 4.2.9).
	Keywords map[string]bool `json:"keywords,omitempty"`
	// Categories is a set of category URIs; every map value MUST be true
	// (Section 4.2.10).
	Categories map[string]bool `json:"categories,omitempty"`
	// Color is a suggested rendering color, a CSS3 color name or value
	// (Section 4.2.11).
	Color string `json:"color,omitempty"`

	// --- Recurrence properties (Section 4.3) ---

	// RecurrenceID is the [LocalDateTime] of the master instance this
	// object overrides; set on a standalone recurrence instance (Section
	// 4.3.1). Pointer so absence is distinguishable from the zero value.
	RecurrenceID *LocalDateTime `json:"recurrenceId,omitempty"`
	// RecurrenceIDTimeZone is the time zone of the master instance
	// (Section 4.3.2). It MUST be set when RecurrenceID is set and MUST
	// NOT be set otherwise.
	RecurrenceIDTimeZone TimeZoneId `json:"recurrenceIdTimeZone,omitempty"`
	// RecurrenceRules generate the recurrence set from Start (Section
	// 4.3.3). Each rule is evaluated against the master object's Start, so
	// the recurrence set is the union of the instances every rule produces.
	// Expanding the rules into concrete occurrences is a consumer concern
	// and out of scope for this library: the model carries the rules
	// verbatim and leaves expansion to the caller.
	RecurrenceRules []RecurrenceRule `json:"recurrenceRules,omitempty"`
	// ExcludedRecurrenceRules subtract instances from the set produced by
	// RecurrenceRules (Section 4.3.4). The excluded rules are evaluated
	// against the same master Start as RecurrenceRules, and the instances
	// they generate are removed from the recurrence set — an instance is
	// part of the set only if some RecurrenceRules rule produces it and no
	// ExcludedRecurrenceRules rule does. As with RecurrenceRules, this
	// subtraction is a semantic the consumer performs during expansion; the
	// library only round-trips the rules.
	ExcludedRecurrenceRules []RecurrenceRule `json:"excludedRecurrenceRules,omitempty"`
	// RecurrenceOverrides maps an occurrence's [LocalDateTime] start (as a
	// string key) to a [PatchObject] that adjusts that one occurrence
	// (Section 4.3.5). Each key is a LocalDateTime equal to the overridden
	// occurrence's start as the recurrence set produces it, expressed in the
	// master object's TimeZone — not UTC and not an arbitrary identifier. The
	// key is a string rather than a typed LocalDateTime because encoding/json
	// map keys must be strings; the key's grammar is validated at the
	// validation boundary.
	//
	// The patch values reuse the [PatchObject] codec: each adjusts the master
	// for that one occurrence. A patch whose "excluded" pointer is set to true
	// (the JSON `{"excluded":true}`) deletes that occurrence from the
	// recurrence set rather than adding a patched instance — the spec's way of
	// removing a single produced occurrence. A patch value of JSON null at a
	// pointer removes the property it addresses (the §3.3 removal sentinel),
	// which is distinct from the whole-occurrence "excluded" deletion. As with
	// the recurrence rules, the library round-trips the overrides verbatim and
	// leaves applying them — and the exclusion — to the consumer's expansion.
	//
	// A standalone, fully expanded override object (one that lives outside this
	// map, e.g. fetched on its own) instead carries [Event.RecurrenceID] and
	// [Event.RecurrenceIDTimeZone] to point back at the occurrence it replaces.
	RecurrenceOverrides map[string]PatchObject `json:"recurrenceOverrides,omitempty"`

	// --- Scheduling properties (Section 4.4) ---

	// Priority is the scheduling priority, 0..9 with 0 undefined (Section
	// 4.4.1); default 0.
	Priority int `json:"priority,omitempty"`
	// FreeBusyStatus is whether the object blocks time, "free" or "busy"
	// (Section 4.4.2); default "busy". The value is open: unrecognized
	// strings round-trip.
	FreeBusyStatus string `json:"freeBusyStatus,omitempty"`
	// Privacy is the access classification, "public", "private", or
	// "secret" (Section 4.4.3); default "public". The value is open.
	Privacy string `json:"privacy,omitempty"`

	// UseDefaultAlerts indicates the user's default alerts should be used
	// in place of the alerts property (Section 4.5.1); default false.
	UseDefaultAlerts bool `json:"useDefaultAlerts,omitempty"`

	// --- Event-specific scheduling properties (Section 5.1) ---

	// Start is the [LocalDateTime] at which the event begins (Section
	// 5.1.1), interpreted in TimeZone (floating when TimeZone is unset).
	Start *LocalDateTime `json:"start,omitempty"`
	// Duration is the event's length as a [Duration] (Section 5.1.2);
	// default "P0D".
	Duration *Duration `json:"duration,omitempty"`
	// Status is the scheduling status of the event, "confirmed",
	// "cancelled", or "tentative" (Section 5.1.3); default "confirmed".
	// The value is open.
	Status string `json:"status,omitempty"`

	// TimeZone is the [TimeZoneId] the object is scheduled in, or unset
	// for floating time (Section 4.7.1).
	TimeZone TimeZoneId `json:"timeZone,omitempty"`

	// TODO(phase 4, JSCAL-18/19/20): participants, locations,
	// virtualLocations, links, relatedTo, alerts, localizations, and the
	// embedded timeZones map. Their value types (Participant, Location,
	// VirtualLocation, Link, Relation, Alert, TimeZone) are added in
	// phase 4; the fields land with them.

	// Extra holds object members with no corresponding known property —
	// vendor extensions and properties from future spec revisions, which
	// are indistinguishable on the wire and must both survive a round
	// trip (RFC 8984 treats unrecognized members as data to preserve, not
	// discard). The codec captures any unknown member here on decode and
	// re-emits it on encode, so unknown input round-trips losslessly. The
	// json:"-" tag keeps the default struct codec away from the field;
	// codec.go splices these members onto the wire after the known
	// properties, in sorted key order, for byte-stable output. Values are
	// kept as [json.RawMessage] for byte-stable, allocation-free
	// preservation — no map[string]any reordering, no deserialize cost
	// for members the consumer never reads. Use [DecodeJSON] and
	// [EncodeJSON] for typed access. A member whose name matches a known
	// property is decoded into that property and is never captured here.
	Extra map[string]json.RawMessage `json:"-"`
}

// Task is a JSCalendar "Task" (RFC 8984, Section 2.2): a to-do that may
// have a [Task.Due] by which it should be completed and, optionally, a
// [Task.Start] and [Task.EstimatedDuration].
//
// As with [Event], only Type and UID are required; all other properties
// are optional and omitted when zero.
type Task struct {
	// Type is the "@type" discriminator and MUST equal "Task" (Section
	// 4.1.1). First declared field so the codec emits it first; the codec
	// forces the value to "Task", so a zero Type still marshals correctly.
	Type string `json:"@type,omitempty"`

	// --- Metadata properties (Section 4.1) ---

	// UID is the globally unique, lifetime-stable identifier (Section
	// 4.1.2).
	UID string `json:"uid,omitempty"`
	// ProdID identifies the creating product (Section 4.1.4).
	ProdID string `json:"prodId,omitempty"`
	// Created is the creation UTC date-time (Section 4.1.5).
	Created *UTCDateTime `json:"created,omitempty"`
	// Updated is the last-modified UTC date-time (Section 4.1.6).
	Updated *UTCDateTime `json:"updated,omitempty"`
	// Sequence is the revision number (Section 4.1.7); default 0.
	Sequence uint `json:"sequence,omitempty"`
	// Method is the iTIP scheduling method (Section 4.1.8).
	Method string `json:"method,omitempty"`

	// --- Descriptive properties (Section 4.2) ---

	// Title is the short summary (Section 4.2.1).
	Title string `json:"title,omitempty"`
	// Description is the longer free-text description (Section 4.2.2).
	Description string `json:"description,omitempty"`
	// DescriptionContentType is the media type of Description (Section
	// 4.2.3).
	DescriptionContentType string `json:"descriptionContentType,omitempty"`
	// ShowWithoutTime indicates the time of day is not significant for
	// display (Section 4.2.4); default false.
	ShowWithoutTime bool `json:"showWithoutTime,omitempty"`
	// Keywords is a set of free-form keywords; values MUST be true
	// (Section 4.2.9).
	Keywords map[string]bool `json:"keywords,omitempty"`
	// Categories is a set of category URIs; values MUST be true (Section
	// 4.2.10).
	Categories map[string]bool `json:"categories,omitempty"`
	// Color is a suggested rendering color (Section 4.2.11).
	Color string `json:"color,omitempty"`

	// --- Recurrence properties (Section 4.3) ---

	// RecurrenceID is the master instance's [LocalDateTime] (Section
	// 4.3.1).
	RecurrenceID *LocalDateTime `json:"recurrenceId,omitempty"`
	// RecurrenceIDTimeZone is the master instance's time zone (Section
	// 4.3.2). Coupled to RecurrenceID per the spec.
	RecurrenceIDTimeZone TimeZoneId `json:"recurrenceIdTimeZone,omitempty"`
	// RecurrenceRules generate the recurrence set from Start (Section
	// 4.3.3). See the [Event.RecurrenceRules] documentation for the full
	// semantics: each rule is evaluated against the master object's Start,
	// and expanding the rules into concrete occurrences is a consumer
	// concern this library leaves to the caller.
	RecurrenceRules []RecurrenceRule `json:"recurrenceRules,omitempty"`
	// ExcludedRecurrenceRules subtract from the recurrence set (Section
	// 4.3.4). See the [Event.ExcludedRecurrenceRules] documentation: the
	// excluded rules are evaluated against the same master Start, and the
	// instances they generate are removed from the set RecurrenceRules
	// produces. The subtraction is performed by the consumer during
	// expansion; the library only round-trips the rules.
	ExcludedRecurrenceRules []RecurrenceRule `json:"excludedRecurrenceRules,omitempty"`
	// RecurrenceOverrides maps an occurrence's [LocalDateTime] start (as a
	// string key) to a [PatchObject] (Section 4.3.5). See the
	// [Event.RecurrenceOverrides] documentation for the full semantics: the
	// key is a LocalDateTime equal to the overridden occurrence's start in the
	// master's TimeZone (a string because encoding/json map keys must be
	// strings), an override that sets "excluded" to true deletes that
	// occurrence, and a standalone override object carries
	// [Task.RecurrenceID] / [Task.RecurrenceIDTimeZone] instead.
	RecurrenceOverrides map[string]PatchObject `json:"recurrenceOverrides,omitempty"`

	// --- Scheduling properties (Section 4.4) ---

	// Priority is the scheduling priority, 0..9 (Section 4.4.1); default
	// 0.
	Priority int `json:"priority,omitempty"`
	// FreeBusyStatus is whether the task blocks time (Section 4.4.2);
	// default "busy". Open value.
	FreeBusyStatus string `json:"freeBusyStatus,omitempty"`
	// Privacy is the access classification (Section 4.4.3); default
	// "public". Open value.
	Privacy string `json:"privacy,omitempty"`

	// UseDefaultAlerts indicates the user's default alerts should be used
	// (Section 4.5.1); default false.
	UseDefaultAlerts bool `json:"useDefaultAlerts,omitempty"`

	// --- Task-specific scheduling properties (Section 5.2) ---

	// Due is the [LocalDateTime] by which the task is due (Section 5.2.1),
	// interpreted in TimeZone.
	Due *LocalDateTime `json:"due,omitempty"`
	// Start is the [LocalDateTime] at which work on the task begins
	// (Section 5.2.2).
	Start *LocalDateTime `json:"start,omitempty"`
	// EstimatedDuration is the estimated effort as a [Duration] (Section
	// 5.2.3).
	EstimatedDuration *Duration `json:"estimatedDuration,omitempty"`
	// PercentComplete is the overall completion, an integer 0..100
	// (Section 5.2.4). It is a pointer so an explicit 0% is distinguished
	// from an absent value.
	PercentComplete *uint `json:"percentComplete,omitempty"`
	// Progress is the task's progress state, e.g. "needs-action",
	// "in-process", "completed", or "failed" (Section 5.2.5). Open value.
	Progress string `json:"progress,omitempty"`

	// TimeZone is the [TimeZoneId] the task is scheduled in, or unset for
	// floating time (Section 4.7.1).
	TimeZone TimeZoneId `json:"timeZone,omitempty"`

	// TODO(phase 4, JSCAL-18/19/20): participants, locations,
	// virtualLocations, links, relatedTo, alerts, localizations, and the
	// embedded timeZones map land with their value types in phase 4.

	// Extra holds object members with no corresponding known property.
	// See the [Event.Extra] documentation for the full semantics: unknown
	// members are captured here on decode and re-emitted on encode so a
	// vendor extension or future-spec property round-trips losslessly and
	// byte-stably. The json:"-" tag reserves the field for the codec. Use
	// [DecodeJSON] and [EncodeJSON] for typed access.
	Extra map[string]json.RawMessage `json:"-"`
}

// Group is a JSCalendar "Group" (RFC 8984, Section 2.3): an ordered
// collection of [Event] and [Task] objects.
//
// Type and UID are required; the spec also makes [Group.Entries]
// mandatory, though it is not enforced on marshal here (that is a
// validation concern). Group carries the metadata and descriptive
// properties but none of the scheduling or recurrence properties, which
// have no meaning for a collection.
type Group struct {
	// Type is the "@type" discriminator and MUST equal "Group" (Section
	// 4.1.1). First declared field so the codec emits it first; the codec
	// forces the value to "Group", so a zero Type still marshals correctly.
	Type string `json:"@type,omitempty"`

	// --- Metadata properties (Section 4.1) ---

	// UID is the globally unique, lifetime-stable identifier (Section
	// 4.1.2).
	UID string `json:"uid,omitempty"`
	// ProdID identifies the creating product (Section 4.1.4).
	ProdID string `json:"prodId,omitempty"`
	// Created is the creation UTC date-time (Section 4.1.5).
	Created *UTCDateTime `json:"created,omitempty"`
	// Updated is the last-modified UTC date-time (Section 4.1.6).
	Updated *UTCDateTime `json:"updated,omitempty"`
	// Sequence is the revision number (Section 4.1.7); default 0.
	Sequence uint `json:"sequence,omitempty"`
	// Method is the iTIP scheduling method (Section 4.1.8).
	Method string `json:"method,omitempty"`

	// --- Descriptive properties (Section 4.2) ---

	// Title is the short summary (Section 4.2.1).
	Title string `json:"title,omitempty"`
	// Description is the longer free-text description (Section 4.2.2).
	Description string `json:"description,omitempty"`
	// DescriptionContentType is the media type of Description (Section
	// 4.2.3).
	DescriptionContentType string `json:"descriptionContentType,omitempty"`
	// Keywords is a set of free-form keywords; values MUST be true
	// (Section 4.2.9).
	Keywords map[string]bool `json:"keywords,omitempty"`
	// Categories is a set of category URIs; values MUST be true (Section
	// 4.2.10).
	Categories map[string]bool `json:"categories,omitempty"`
	// Color is a suggested rendering color (Section 4.2.11).
	Color string `json:"color,omitempty"`

	// --- Group-specific properties (Section 5.3) ---

	// Entries is the collection of group members (Section 5.3.1). Each
	// entry is an [Event] or [Task]; it is retained as [json.RawMessage]
	// for now so the exact member bytes round-trip. Typed dispatch on the
	// member "@type" arrives with Parse in a later phase.
	Entries []json.RawMessage `json:"entries,omitempty"`
	// Source is the URI from which updated versions of the group may be
	// retrieved (Section 5.3.2).
	Source string `json:"source,omitempty"`

	// TODO(phase 4, JSCAL-19/20): links, relatedTo, localizations, and the
	// embedded timeZones map land with their value types in phase 4.

	// Extra holds object members with no corresponding known property.
	// See the [Event.Extra] documentation for the full semantics: unknown
	// members are captured here on decode and re-emitted on encode so a
	// vendor extension or future-spec property round-trips losslessly and
	// byte-stably. The json:"-" tag reserves the field for the codec. Use
	// [DecodeJSON] and [EncodeJSON] for typed access.
	Extra map[string]json.RawMessage `json:"-"`
}

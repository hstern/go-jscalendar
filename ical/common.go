// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package ical

import (
	"fmt"
	"strconv"
	"strings"

	goical "github.com/emersion/go-ical"

	"github.com/hstern/go-jscalendar"
)

// This file maps the iCalendar properties that an Event and a Task share — the
// metadata, descriptive, and sub-object properties common to both — onto the
// JSCalendar fields of either. The shared work is threaded through commonTarget,
// a bundle of pointers to the destination fields, so eventFromVEvent and
// taskFromVTodo express only their type-specific differences (start/duration
// vs. due/percent-complete) and delegate the rest here.

// commonTarget bundles pointers to the JSCalendar fields shared by [Event] and
// [Task] that commonProps fills. Each pointer addresses the corresponding field
// of the concrete object being built, so a single mapping populates either type.
type commonTarget struct {
	uid          *string
	title        *string
	description  *string
	sequence     *uint
	created      **jscalendar.UTCDateTime
	updated      **jscalendar.UTCDateTime
	privacy      *string
	keywords     *map[string]bool
	locations    *map[jscalendar.Id]jscalendar.Location
	participants *map[jscalendar.Id]jscalendar.Participant
	links        *map[jscalendar.Id]jscalendar.Link
	alerts       *map[jscalendar.Id]jscalendar.Alert
}

// commonProps maps the properties shared by VEVENT and VTODO onto the fields
// addressed by dst:
//
//   - UID, SUMMARY → title, DESCRIPTION, SEQUENCE, CLASS → privacy (lower-cased)
//   - CREATED → created; LAST-MODIFIED, else DTSTAMP, → updated
//   - CATEGORIES → keywords (the set form; every value true)
//   - LOCATION → a single Location keyed "1"
//   - ORGANIZER and ATTENDEE → participants
//   - URL → a single Link keyed "1"
//   - each VALARM child → an Alert
func commonProps(comp *goical.Component, dst *commonTarget) error {
	*dst.uid = textValue(comp.Props, goical.PropUID)
	*dst.title = textValue(comp.Props, goical.PropSummary)
	*dst.description = textValue(comp.Props, goical.PropDescription)

	if class := rawValue(comp.Props, goical.PropClass); class != "" {
		*dst.privacy = privacyFromClass(class)
	}

	if seq := rawValue(comp.Props, goical.PropSequence); seq != "" {
		n, err := strconv.ParseUint(seq, 10, 64)
		if err != nil {
			return fmt.Errorf("ical: malformed SEQUENCE %q: %w", seq, err)
		}
		*dst.sequence = uint(n)
	}

	created, err := utcDateTimeProp(comp.Props, goical.PropCreated)
	if err != nil {
		return err
	}
	*dst.created = created

	updated, err := updatedProp(comp.Props)
	if err != nil {
		return err
	}
	*dst.updated = updated

	*dst.keywords = keywordsFromCategories(comp.Props)
	*dst.locations = locationsFromProp(comp.Props)
	*dst.participants = participantsFromProps(comp.Props)
	*dst.links = linksFromProp(comp.Props)

	alerts, err := alertsFromVAlarms(comp)
	if err != nil {
		return err
	}
	*dst.alerts = alerts

	return nil
}

// privacyFromClass maps an iCalendar CLASS value to a JSCalendar privacy value.
// RFC 5545's "PUBLIC" / "PRIVATE" / "CONFIDENTIAL" correspond to JSCalendar's
// "public" / "private" / "secret" (RFC 8984, Section 4.4.3); an unregistered
// CLASS is lower-cased and carried across, since privacy is an open value.
func privacyFromClass(class string) string {
	switch strings.ToUpper(class) {
	case "PUBLIC":
		return "public"
	case "PRIVATE":
		return "private"
	case "CONFIDENTIAL":
		return "secret"
	default:
		return strings.ToLower(class)
	}
}

// utcDateTimeProp parses a UTC DATE-TIME property (CREATED, LAST-MODIFIED,
// DTSTAMP) into a [*jscalendar.UTCDateTime], or nil when the property is absent.
// These properties are defined as UTC in RFC 5545, so the wall clock is taken
// directly as the UTC instant.
func utcDateTimeProp(props goical.Props, name string) (*jscalendar.UTCDateTime, error) {
	prop := props.Get(name)
	if prop == nil {
		return nil, nil
	}
	local, err := parseICalDateTime(prop.Value)
	if err != nil {
		return nil, fmt.Errorf("ical: %s: %w", name, err)
	}
	// LocalDateTime and UTCDateTime share an identical field layout; the source
	// property is defined as UTC by RFC 5545, so the wall clock is the UTC
	// instant directly.
	utc := jscalendar.UTCDateTime(local)
	return &utc, nil
}

// updatedProp derives the JSCalendar updated timestamp: LAST-MODIFIED when
// present, otherwise DTSTAMP. Both are UTC DATE-TIME properties (RFC 5545); the
// calext mapping prefers LAST-MODIFIED, falling back to the always-present
// DTSTAMP.
func updatedProp(props goical.Props) (*jscalendar.UTCDateTime, error) {
	if props.Get(goical.PropLastModified) != nil {
		return utcDateTimeProp(props, goical.PropLastModified)
	}
	return utcDateTimeProp(props, goical.PropDateTimeStamp)
}

// keywordsFromCategories maps the CATEGORIES property (a comma-separated TEXT
// list, possibly repeated) onto the JSCalendar keywords set, where every value
// is true (RFC 8984, Section 4.2.9). It returns nil when there are no
// categories.
func keywordsFromCategories(props goical.Props) map[string]bool {
	var keywords map[string]bool
	for _, prop := range props.Values(goical.PropCategories) {
		list, err := prop.TextList()
		if err != nil {
			list = []string{prop.Value}
		}
		for _, cat := range list {
			if cat == "" {
				continue
			}
			if keywords == nil {
				keywords = map[string]bool{}
			}
			keywords[cat] = true
		}
	}
	return keywords
}

// locationsFromProp maps the LOCATION property onto a single JSCalendar
// [jscalendar.Location] keyed "1". iCalendar carries at most one LOCATION text;
// JSCalendar's locations is a keyed map, so the single location takes the stable
// id "1". It returns nil when there is no LOCATION.
func locationsFromProp(props goical.Props) map[jscalendar.Id]jscalendar.Location {
	name := textValue(props, goical.PropLocation)
	if name == "" {
		return nil
	}
	return map[jscalendar.Id]jscalendar.Location{
		"1": {Type: "Location", Name: name},
	}
}

// linksFromProp maps the URL property onto a single JSCalendar [jscalendar.Link]
// keyed "1". It returns nil when there is no URL.
func linksFromProp(props goical.Props) map[jscalendar.Id]jscalendar.Link {
	href := rawValue(props, goical.PropURL)
	if href == "" {
		return nil
	}
	return map[jscalendar.Id]jscalendar.Link{
		"1": {Type: "Link", Href: href},
	}
}

// participantsFromProps maps ORGANIZER and ATTENDEE properties onto JSCalendar
// participants. The ORGANIZER becomes a participant with the "owner" role; each
// ATTENDEE becomes one with the "attendee" role. Both carry the CAL-ADDRESS as
// the participant's sendTo "imip" entry and, when the address is a mailto URI,
// its email; the CN parameter becomes the name. Participants are keyed by a
// generated ordinal id. It returns nil when there are neither.
func participantsFromProps(props goical.Props) map[jscalendar.Id]jscalendar.Participant {
	var participants map[jscalendar.Id]jscalendar.Participant
	add := func(id jscalendar.Id, prop goical.Prop, role string) {
		if participants == nil {
			participants = map[jscalendar.Id]jscalendar.Participant{}
		}
		participants[id] = participantFromCalAddress(prop, role)
	}

	if organizer := props.Get(goical.PropOrganizer); organizer != nil {
		add("1", *organizer, "owner")
	}
	for i, attendee := range props.Values(goical.PropAttendee) {
		add(jscalendar.Id(strconv.Itoa(i+2)), attendee, "attendee")
	}
	return participants
}

// participantFromCalAddress builds a [jscalendar.Participant] from an ORGANIZER
// or ATTENDEE property and the role it plays. The CAL-ADDRESS value is the
// scheduling address (sendTo "imip"); a "mailto:" address additionally fills
// email. The CN parameter, when present, is the display name.
func participantFromCalAddress(prop goical.Prop, role string) jscalendar.Participant {
	p := jscalendar.Participant{
		Type:  "Participant",
		Roles: map[string]bool{role: true},
	}
	if cn := prop.Params.Get(goical.ParamCommonName); cn != "" {
		p.Name = cn
	}
	addr := prop.Value
	if addr != "" {
		p.SendTo = map[string]string{"imip": addr}
		if email, ok := strings.CutPrefix(addr, "mailto:"); ok {
			p.Email = email
		}
	}
	return p
}

// alertsFromVAlarms maps a component's VALARM children onto JSCalendar alerts,
// keyed by a generated ordinal id. It returns nil when there are no alarms.
func alertsFromVAlarms(comp *goical.Component) (map[jscalendar.Id]jscalendar.Alert, error) {
	var alerts map[jscalendar.Id]jscalendar.Alert
	n := 0
	for _, child := range comp.Children {
		if child.Name != goical.CompAlarm {
			continue
		}
		alert, err := alertFromVAlarm(child)
		if err != nil {
			return nil, err
		}
		n++
		if alerts == nil {
			alerts = map[jscalendar.Id]jscalendar.Alert{}
		}
		alerts[jscalendar.Id(strconv.Itoa(n))] = alert
	}
	return alerts, nil
}

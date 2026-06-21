// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package ical

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	goical "github.com/emersion/go-ical"

	"github.com/hstern/go-jscalendar"
)

// This file holds the value-level conversions that ToICal shares across the
// VEVENT and VTODO mappings: the LocalDateTime/timeZone recomposition that is
// the mirror of from_ical.go's split, the duration, recurrence-rule, alarm, and
// VTIMEZONE constructions, and the common-property writer. Each is the inverse
// of a from_ical.go helper, and reuses the same fixed RFC 5545 layouts and the
// same UTC-offset shape so the two directions stay symmetric.

// dateTimeProp recomposes a JSCalendar LocalDateTime plus its zone back into an
// iCalendar DATE / DATE-TIME property — the inverse of splitDateTimeProp:
//
//   - An all-day value (showWithoutTime, no zone) becomes a DATE value with
//     VALUE=DATE.
//   - The "Etc/UTC" zone becomes a UTC DATE-TIME with a trailing "Z".
//   - Any other zone becomes a DATE-TIME carrying a ";TZID=" parameter, with the
//     custom "/"-prefixed identifier stripped back to its bare TZID (the form
//     iCalendar uses; the leading "/" is JSCalendar's custom-zone marker).
//   - A floating value (no zone) becomes a bare DATE-TIME.
func dateTimeProp(name string, lt jscalendar.LocalDateTime, zone jscalendar.TimeZoneId, allDay bool) *goical.Prop {
	prop := goical.NewProp(name)

	if allDay && zone == "" {
		prop.SetValueType(goical.ValueDate)
		prop.Value = formatICalDate(lt)
		return prop
	}

	switch {
	case zone == utcTimeZone:
		prop.Value = formatICalDateTime(lt) + "Z"
	case zone != "":
		prop.Params.Set(goical.ParamTimezoneID, bareTZID(zone))
		prop.Value = formatICalDateTime(lt)
	default:
		prop.Value = formatICalDateTime(lt)
	}
	return prop
}

// bareTZID returns the iCalendar TZID for a JSCalendar TimeZoneId: a custom
// ("/"-prefixed) identifier has its leading "/" removed, since the VTIMEZONE it
// resolves to is keyed by the bare TZID; an IANA name is returned unchanged.
func bareTZID(zone jscalendar.TimeZoneId) string {
	return strings.TrimPrefix(string(zone), "/")
}

// formatICalDate formats a LocalDateTime's date part as an RFC 5545 DATE value
// ("YYYYMMDD"), the inverse of parseICalDate.
func formatICalDate(lt jscalendar.LocalDateTime) string {
	return fmt.Sprintf("%04d%02d%02d", lt.Year, lt.Month, lt.Day)
}

// formatICalDateTime formats a LocalDateTime as an RFC 5545 DATE-TIME value
// ("YYYYMMDDTHHMMSS"), without any zone designator, the inverse of
// parseICalDateTime. A trailing "Z" or a ";TZID=" parameter is the caller's
// concern (see dateTimeProp), keeping the zone separate from the wall clock.
func formatICalDateTime(lt jscalendar.LocalDateTime) string {
	return fmt.Sprintf("%04d%02d%02dT%02d%02d%02d",
		lt.Year, lt.Month, lt.Day, lt.Hour, lt.Minute, lt.Second)
}

// formatICalUTC formats a UTCDateTime as a UTC DATE-TIME value with the trailing
// "Z" RFC 5545 requires for CREATED, LAST-MODIFIED, DTSTAMP, and an absolute
// TRIGGER. It is the inverse of utcDateTimeProp's read.
func formatICalUTC(ut jscalendar.UTCDateTime) string {
	return fmt.Sprintf("%04d%02d%02dT%02d%02d%02dZ",
		ut.Year, ut.Month, ut.Day, ut.Hour, ut.Minute, ut.Second)
}

// durationProp builds a DURATION property from a JSCalendar Duration. The
// JSCalendar Duration grammar is a superset of the iCalendar one; the forms a
// VEVENT/VTODO span uses (weeks, days, H/M/S) render identically, so the
// Duration's own String is the iCalendar value.
func durationProp(d jscalendar.Duration) *goical.Prop {
	prop := goical.NewProp(goical.PropDuration)
	prop.Value = d.String()
	return prop
}

// rruleString renders a JSCalendar RecurrenceRule back into an iCalendar RRULE
// value string — the inverse of recurrenceRuleFromRRule. The parts are emitted
// in the canonical RFC 5545 order with FREQ first; a rule with no Frequency is
// rejected, mirroring the strict-marshal boundary the JSCalendar codec applies.
func rruleString(rule jscalendar.RecurrenceRule) (string, error) {
	if rule.Frequency == "" {
		return "", fmt.Errorf("ical: recurrence rule has no frequency")
	}

	var parts []string
	add := func(key, val string) {
		if val != "" {
			parts = append(parts, key+"="+val)
		}
	}

	add("FREQ", strings.ToUpper(string(rule.Frequency)))
	if rule.Interval != 0 {
		// Emit any explicitly-set interval, including the redundant INTERVAL=1, so
		// a rule that carried it round-trips verbatim. An unset interval (the zero
		// value, meaning the default of 1) is omitted.
		add("INTERVAL", strconv.FormatUint(uint64(rule.Interval), 10))
	}
	if rule.RScale != "" {
		add("RSCALE", strings.ToUpper(rule.RScale))
	}
	if rule.Skip != "" {
		add("SKIP", strings.ToUpper(rule.Skip))
	}
	if rule.FirstDayOfWeek != "" {
		add("WKST", strings.ToUpper(rule.FirstDayOfWeek))
	}
	add("BYDAY", formatByDay(rule.ByDay))
	add("BYMONTHDAY", joinInts(rule.ByMonthDay))
	add("BYMONTH", strings.Join(rule.ByMonth, ","))
	add("BYYEARDAY", joinInts(rule.ByYearDay))
	add("BYWEEKNO", joinInts(rule.ByWeekNo))
	add("BYHOUR", joinUints(rule.ByHour))
	add("BYMINUTE", joinUints(rule.ByMinute))
	add("BYSECOND", joinUints(rule.BySecond))
	add("BYSETPOS", joinInts(rule.BySetPosition))
	if rule.Count != nil {
		add("COUNT", strconv.FormatUint(uint64(*rule.Count), 10))
	}
	if rule.Until != nil {
		// UNTIL is a JSCalendar LocalDateTime interpreted in the object's zone; it
		// is written back as a bare DATE-TIME with no zone designator, the inverse
		// of parseUntil's zone-designator drop.
		add("UNTIL", formatICalDateTime(*rule.Until))
	}

	return strings.Join(parts, ";"), nil
}

// formatByDay renders a BYDAY list of JSCalendar NDay values back into the RFC
// 5545 comma-separated form ("MO", "-1SU", "2TH"): the optional NthOfPeriod
// ordinal precedes the upper-cased two-letter weekday code, the inverse of
// parseNDay.
func formatByDay(days []jscalendar.NDay) string {
	if len(days) == 0 {
		return ""
	}
	parts := make([]string, 0, len(days))
	for _, d := range days {
		var b strings.Builder
		if d.NthOfPeriod != nil {
			b.WriteString(strconv.Itoa(*d.NthOfPeriod))
		}
		b.WriteString(strings.ToUpper(d.Day))
		parts = append(parts, b.String())
	}
	return strings.Join(parts, ",")
}

// joinInts renders a signed-integer RRULE list (BYMONTHDAY, BYYEARDAY,
// BYWEEKNO, BYSETPOS) as a comma-separated string.
func joinInts(values []int) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, ",")
}

// joinUints renders a non-negative-integer RRULE list (BYHOUR, BYMINUTE,
// BYSECOND) as a comma-separated string.
func joinUints(values []uint) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = strconv.FormatUint(uint64(v), 10)
	}
	return strings.Join(parts, ",")
}

// commonSource bundles the JSCalendar fields shared by Event and Task that
// commonPropsToComp writes onto a component — the mirror of commonTarget.
type commonSource struct {
	uid          string
	title        string
	description  string
	sequence     uint
	created      *jscalendar.UTCDateTime
	updated      *jscalendar.UTCDateTime
	privacy      string
	keywords     map[string]bool
	locations    map[jscalendar.Id]jscalendar.Location
	participants map[jscalendar.Id]jscalendar.Participant
	links        map[jscalendar.Id]jscalendar.Link
	alerts       map[jscalendar.Id]jscalendar.Alert
}

// commonPropsToComp writes the properties shared by VEVENT and VTODO from src —
// the inverse of commonProps. UID and DTSTAMP are always written so the result
// satisfies the encoder's exactly-one requirement: DTSTAMP takes the updated
// timestamp when present, falling back to a fixed epoch only when the object
// carries none (FromICal always populates updated, so the fallback is for
// hand-built objects).
func commonPropsToComp(comp *goical.Component, src *commonSource) error {
	comp.Props.SetText(goical.PropUID, src.uid)

	if src.title != "" {
		comp.Props.SetText(goical.PropSummary, src.title)
	}
	if src.description != "" {
		comp.Props.SetText(goical.PropDescription, src.description)
	}
	if src.sequence != 0 {
		// SEQUENCE is an INTEGER property; set the raw value rather than going
		// through SetText, which would stamp a spurious VALUE=TEXT parameter.
		seq := goical.NewProp(goical.PropSequence)
		seq.Value = strconv.FormatUint(uint64(src.sequence), 10)
		comp.Props.Set(seq)
	}
	if src.privacy != "" {
		comp.Props.SetText(goical.PropClass, classFromPrivacy(src.privacy))
	}
	if src.created != nil {
		prop := goical.NewProp(goical.PropCreated)
		prop.Value = formatICalUTC(*src.created)
		comp.Props.Set(prop)
	}

	// DTSTAMP is mandatory. LAST-MODIFIED mirrors updated when present; DTSTAMP
	// also takes updated (the property FromICal reads back into updated). When no
	// updated is set, DTSTAMP falls back to a fixed, conformant placeholder so
	// the encoder's exactly-one DTSTAMP requirement is met.
	stamp := defaultDTStamp
	if src.updated != nil {
		stamp = formatICalUTC(*src.updated)
		lm := goical.NewProp(goical.PropLastModified)
		lm.Value = stamp
		comp.Props.Set(lm)
	}
	dtstamp := goical.NewProp(goical.PropDateTimeStamp)
	dtstamp.Value = stamp
	comp.Props.Set(dtstamp)

	if categories := sortedTrueKeys(src.keywords); len(categories) > 0 {
		prop := goical.NewProp(goical.PropCategories)
		prop.SetTextList(categories)
		comp.Props.Set(prop)
	}

	writeLocation(comp, src.locations)
	writeURL(comp, src.links)
	writeParticipants(comp, src.participants)

	return alertsToVAlarms(comp, src.alerts)
}

// defaultDTStamp is the placeholder DTSTAMP used only when an object carries no
// updated timestamp. It is a valid UTC DATE-TIME so the encoder accepts it; a
// converted object always supplies its own, so this is reached only for
// hand-built inputs.
const defaultDTStamp = "19700101T000000Z"

// classFromPrivacy maps a JSCalendar privacy value back to an iCalendar CLASS —
// the inverse of privacyFromClass. The three registered values round-trip
// exactly; any other (open) value is upper-cased and carried across.
func classFromPrivacy(privacy string) string {
	switch strings.ToLower(privacy) {
	case "public":
		return "PUBLIC"
	case "private":
		return "PRIVATE"
	case "secret":
		return "CONFIDENTIAL"
	default:
		return strings.ToUpper(privacy)
	}
}

// writeLocation writes a single LOCATION from the first location in id order.
// iCalendar carries at most one LOCATION text, so a multi-location JSCalendar
// object is lossy here (documented on ToICal); the lowest id wins so the choice
// is deterministic.
func writeLocation(comp *goical.Component, locations map[jscalendar.Id]jscalendar.Location) {
	for _, id := range sortedIDs(locations) {
		if name := locations[id].Name; name != "" {
			comp.Props.SetText(goical.PropLocation, name)
			return
		}
	}
}

// writeURL writes a single URL from the first link in id order. iCalendar
// carries one URL; additional links have no place and are lossy (documented on
// ToICal).
func writeURL(comp *goical.Component, links map[jscalendar.Id]jscalendar.Link) {
	for _, id := range sortedIDs(links) {
		if href := links[id].Href; href != "" {
			prop := goical.NewProp(goical.PropURL)
			prop.Value = href
			comp.Props.Set(prop)
			return
		}
	}
}

// writeParticipants writes ORGANIZER and ATTENDEE properties from the
// participants map — the inverse of participantsFromProps. A participant with
// the "owner" role becomes the (single) ORGANIZER; every other participant
// becomes an ATTENDEE. The scheduling address is the sendTo "imip" entry, or a
// "mailto:" of the email when imip is absent; a name becomes the CN parameter.
func writeParticipants(comp *goical.Component, participants map[jscalendar.Id]jscalendar.Participant) {
	organizerSet := false
	for _, id := range sortedIDs(participants) {
		p := participants[id]
		addr := participantAddress(p)
		if addr == "" {
			continue
		}
		if p.Roles["owner"] && !organizerSet {
			prop := calAddressProp(goical.PropOrganizer, addr, p.Name)
			comp.Props.Set(prop)
			organizerSet = true
			continue
		}
		comp.Props.Add(calAddressProp(goical.PropAttendee, addr, p.Name))
	}
}

// participantAddress returns a participant's scheduling address: the sendTo
// "imip" entry, or a "mailto:" form of the email as a fallback when no imip
// method is present.
func participantAddress(p jscalendar.Participant) string {
	if addr := p.SendTo["imip"]; addr != "" {
		return addr
	}
	if p.Email != "" {
		return "mailto:" + p.Email
	}
	return ""
}

// calAddressProp builds an ORGANIZER or ATTENDEE property from a CAL-ADDRESS and
// an optional common name.
func calAddressProp(name, addr, cn string) *goical.Prop {
	prop := goical.NewProp(name)
	prop.Value = addr
	if cn != "" {
		prop.Params.Set(goical.ParamCommonName, cn)
	}
	return prop
}

// sortedTrueKeys returns the keys of a set map (every value true) in sorted
// order, dropping any false entry. It is used for CATEGORIES so the output is
// deterministic.
func sortedTrueKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for k, v := range set {
		if v {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys
}

// sortedIDs returns the [jscalendar.Id] keys of a map in sorted order, so the
// single-valued LOCATION / URL / ORGANIZER choices and the ATTENDEE ordering
// are deterministic across a round trip.
func sortedIDs[V any](m map[jscalendar.Id]V) []jscalendar.Id {
	ids := make([]jscalendar.Id, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

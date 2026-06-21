// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package recur

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	jscalendar "github.com/hstern/go-jscalendar"
)

// Occurrences expands a recurring [jscalendar.Event] into the concrete
// instances whose recurrence id falls in the half-open window [from, until).
//
// The window bound is the occurrence's recurrence id — its nominal start as the
// recurrence rules produce it, resolved to an instant in the event's time zone
// — and not the possibly-overridden start an override may move it to. An
// instance is included when its recurrence id is at or after from and strictly
// before until; an instance exactly at until is excluded.
//
// Each returned event is an independent, fully expanded instance:
//
//   - "start" is the occurrence's start (the recurrence id, unless an override
//     moves it), and "recurrenceId" / "recurrenceIdTimeZone" point back at the
//     instance within the series.
//   - the series-level "recurrenceRules", "excludedRecurrenceRules", and
//     "recurrenceOverrides" are stripped — an instance does not itself recur.
//   - the matching [jscalendar.Event.RecurrenceOverrides] patch, if any, is
//     applied; an override carrying {"excluded": true} drops the instance.
//
// The recurrence set is the union of the instances of every "recurrenceRules"
// rule, minus those of every "excludedRecurrenceRules" rule (RFC 8984,
// Sections 4.3.3 and 4.3.4), with override-only instances (a recurrence id that
// appears in "recurrenceOverrides" but no rule, Section 4.3.5) added. An event
// with neither rules nor overrides yields its single start as one occurrence,
// without a recurrence id, when that start lies in the window.
//
// Occurrences returns an error when the event has no "start" to anchor the
// series, when a time zone cannot be resolved, or when a rule uses a feature
// this cut does not implement (see the package documentation for the RSCALE
// limitation).
func Occurrences(e *jscalendar.Event, from, until time.Time) ([]*jscalendar.Event, error) {
	if e == nil {
		return nil, errors.New("recur: nil event")
	}
	if e.Start == nil {
		return nil, errors.New("recur: event has no start to anchor recurrence")
	}

	masterJSON, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("recur: marshal master event: %w", err)
	}

	exp, err := expandSeries(seriesInput{
		start:     *e.Start,
		timeZone:  e.TimeZone,
		timeZones: e.TimeZones,
		rules:     e.RecurrenceRules,
		excluded:  e.ExcludedRecurrenceRules,
		overrides: e.RecurrenceOverrides,
	}, from, until)
	if err != nil {
		return nil, err
	}

	out := make([]*jscalendar.Event, 0, len(exp.recurrenceIDs))
	for _, rid := range exp.recurrenceIDs {
		occJSON, err := buildOccurrence(masterJSON, rid, e.TimeZone, exp.recurs, exp.patch(rid))
		if err != nil {
			return nil, err
		}
		var occ jscalendar.Event
		if err := json.Unmarshal(occJSON, &occ); err != nil {
			return nil, fmt.Errorf("recur: decode expanded event: %w", err)
		}
		out = append(out, &occ)
	}
	return out, nil
}

// TaskOccurrences expands a recurring [jscalendar.Task] over [from, until). It
// mirrors [Occurrences] in every respect, anchoring the series on the task's
// "start" (RFC 8984, Section 4.3.3); a task with no "start" cannot be expanded
// and returns an error. "due" and "estimatedDuration" ride along on each
// instance unchanged unless an override patch adjusts them.
func TaskOccurrences(t *jscalendar.Task, from, until time.Time) ([]*jscalendar.Task, error) {
	if t == nil {
		return nil, errors.New("recur: nil task")
	}
	if t.Start == nil {
		return nil, errors.New("recur: task has no start to anchor recurrence")
	}

	masterJSON, err := json.Marshal(t)
	if err != nil {
		return nil, fmt.Errorf("recur: marshal master task: %w", err)
	}

	exp, err := expandSeries(seriesInput{
		start:     *t.Start,
		timeZone:  t.TimeZone,
		timeZones: t.TimeZones,
		rules:     t.RecurrenceRules,
		excluded:  t.ExcludedRecurrenceRules,
		overrides: t.RecurrenceOverrides,
	}, from, until)
	if err != nil {
		return nil, err
	}

	out := make([]*jscalendar.Task, 0, len(exp.recurrenceIDs))
	for _, rid := range exp.recurrenceIDs {
		occJSON, err := buildOccurrence(masterJSON, rid, t.TimeZone, exp.recurs, exp.patch(rid))
		if err != nil {
			return nil, err
		}
		var occ jscalendar.Task
		if err := json.Unmarshal(occJSON, &occ); err != nil {
			return nil, fmt.Errorf("recur: decode expanded task: %w", err)
		}
		out = append(out, &occ)
	}
	return out, nil
}

// seriesInput is the recurrence-bearing subset of an Event or Task that
// expandSeries needs, so the expansion runs once for both object types.
type seriesInput struct {
	start     jscalendar.LocalDateTime
	timeZone  jscalendar.TimeZoneId
	timeZones map[jscalendar.TimeZoneId]jscalendar.TimeZone
	rules     []jscalendar.RecurrenceRule
	excluded  []jscalendar.RecurrenceRule
	overrides map[string]jscalendar.PatchObject
}

// expansion is the result of evaluating a series over a window: the ordered
// recurrence ids that survive in [from, until) and the override patch (if any)
// to apply to each.
type expansion struct {
	recurrenceIDs []jscalendar.LocalDateTime
	recurs        bool
	patches       map[string]jscalendar.PatchObject
}

// patch returns the override patch for a recurrence id, or nil if the instance
// is a plain rule-produced occurrence with no override.
func (e expansion) patch(rid jscalendar.LocalDateTime) jscalendar.PatchObject {
	return e.patches[rid.String()]
}

// expandSeries evaluates the recurrence rules, exclusions, and overrides of a
// series against the window and returns the surviving recurrence ids in order.
func expandSeries(in seriesInput, from, until time.Time) (expansion, error) {
	loc, err := resolveLocation(in.timeZone, in.timeZones)
	if err != nil {
		return expansion{}, err
	}
	dtstart := localToTime(in.start, loc)
	recurs := len(in.rules) > 0 || len(in.overrides) > 0

	// produced maps a canonical recurrence-id string to its LocalDateTime, the
	// union of every rule's instances within the window.
	produced := map[string]jscalendar.LocalDateTime{}

	if len(in.rules) == 0 {
		// No rules: the master start is the single regular instance. Override
		// keys may still add or remove instances below.
		produced[in.start.String()] = in.start
	}
	for _, rule := range in.rules {
		rr, err := buildRRule(rule, dtstart)
		if err != nil {
			return expansion{}, err
		}
		for _, t := range rr.Between(from, until, true) {
			if !inWindow(t, from, until) {
				continue
			}
			lt := timeToLocal(t)
			produced[lt.String()] = lt
		}
	}

	// Subtract excluded rules (Section 4.3.4).
	for _, rule := range in.excluded {
		rr, err := buildRRule(rule, dtstart)
		if err != nil {
			return expansion{}, err
		}
		for _, t := range rr.Between(from, until, true) {
			delete(produced, timeToLocal(t).String())
		}
	}

	patches, err := applyOverrideKeys(in.overrides, produced)
	if err != nil {
		return expansion{}, err
	}

	// Clip the final set to the half-open window on the recurrence id.
	ids := make([]jscalendar.LocalDateTime, 0, len(produced))
	for _, lt := range produced {
		if inWindow(localToTime(lt, loc), from, until) {
			ids = append(ids, lt)
		}
	}
	slices.SortFunc(ids, func(a, b jscalendar.LocalDateTime) int {
		return localToTime(a, loc).Compare(localToTime(b, loc))
	})

	return expansion{recurrenceIDs: ids, recurs: recurs, patches: patches}, nil
}

// applyOverrideKeys folds the recurrenceOverrides map into the produced set: an
// {"excluded": true} override removes its instance, any other override adds its
// recurrence id (so an override can introduce an instance the rules do not
// produce, Section 4.3.5) and is collected for later application. The returned
// map is keyed by canonical recurrence-id string.
func applyOverrideKeys(
	overrides map[string]jscalendar.PatchObject,
	produced map[string]jscalendar.LocalDateTime,
) (map[string]jscalendar.PatchObject, error) {
	patches := map[string]jscalendar.PatchObject{}
	for key, patch := range overrides {
		lt, err := jscalendar.ParseLocalDateTime(key)
		if err != nil {
			return nil, fmt.Errorf("recur: recurrenceOverrides key %q: %w", key, err)
		}
		canon := lt.String()
		if isExcludedOverride(patch) {
			delete(produced, canon)
			continue
		}
		produced[canon] = lt
		patches[canon] = patch
	}
	return patches, nil
}

// isExcludedOverride reports whether an override patch deletes its instance —
// that is, whether it sets the "excluded" member to JSON true (Section 4.3.5).
func isExcludedOverride(patch jscalendar.PatchObject) bool {
	raw, ok := patch["excluded"]
	if !ok {
		return false
	}
	var excluded bool
	if err := json.Unmarshal(raw, &excluded); err != nil {
		return false
	}
	return excluded
}

// buildOccurrence renders one expanded instance as JSON: it strips the
// series-level recurrence properties, sets the instance's start and (when the
// object recurs) its recurrence id and recurrence-id time zone, and applies the
// override patch last so an override may move the start.
func buildOccurrence(
	masterJSON []byte,
	rid jscalendar.LocalDateTime,
	timeZone jscalendar.TimeZoneId,
	recurs bool,
	patch jscalendar.PatchObject,
) ([]byte, error) {
	var root map[string]any
	if err := json.Unmarshal(masterJSON, &root); err != nil {
		return nil, fmt.Errorf("recur: decode master: %w", err)
	}

	delete(root, "recurrenceRules")
	delete(root, "excludedRecurrenceRules")
	delete(root, "recurrenceOverrides")

	ridStr := rid.String()
	root["start"] = ridStr
	if recurs {
		root["recurrenceId"] = ridStr
		if timeZone != "" {
			root["recurrenceIdTimeZone"] = string(timeZone)
		}
	}

	occJSON, err := json.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("recur: encode occurrence: %w", err)
	}
	if len(patch) > 0 {
		occJSON, err = ApplyPatch(occJSON, patch)
		if err != nil {
			return nil, err
		}
	}
	return occJSON, nil
}

// resolveLocation resolves a JSCalendar [jscalendar.TimeZoneId] to a Go
// [*time.Location].
//
// An empty id is floating time: it resolves to UTC for the purpose of placing
// instances on the absolute timeline so the window can be applied, while the
// instances themselves stay floating wall-clock values (Section 1.4.4). An IANA
// name resolves through [time.LoadLocation]. A custom, "/"-prefixed id resolves
// against the object's embedded "timeZones": a single-offset definition becomes
// a fixed zone; a definition with daylight-saving transitions is not supported
// and returns an error rather than a silently wrong fixed offset.
func resolveLocation(
	id jscalendar.TimeZoneId,
	zones map[jscalendar.TimeZoneId]jscalendar.TimeZone,
) (*time.Location, error) {
	if id == "" {
		return time.UTC, nil
	}
	if !id.IsCustom() {
		loc, err := time.LoadLocation(string(id))
		if err != nil {
			return nil, fmt.Errorf("recur: load time zone %q: %w", id, err)
		}
		return loc, nil
	}

	def, ok := zones[id]
	if !ok {
		return nil, fmt.Errorf("recur: custom time zone %q is not defined in timeZones", id)
	}
	return fixedZoneFrom(id, def)
}

// fixedZoneFrom builds a fixed-offset [*time.Location] from a custom TimeZone
// whose rules all share one UTC offset. A zone with differing offsets
// (daylight-saving transitions) cannot be represented as a fixed zone and is
// rejected.
func fixedZoneFrom(id jscalendar.TimeZoneId, def jscalendar.TimeZone) (*time.Location, error) {
	var (
		offset int
		name   string
		set    bool
	)
	rules := slices.Concat(def.Standard, def.Daylight)
	if len(rules) == 0 {
		return nil, fmt.Errorf("recur: custom time zone %q has no transition rules", id)
	}
	for _, rule := range rules {
		secs, err := parseUTCOffset(rule.OffsetTo)
		if err != nil {
			return nil, fmt.Errorf("recur: custom time zone %q: %w", id, err)
		}
		if set && secs != offset {
			return nil, fmt.Errorf(
				"recur: custom time zone %q has daylight-saving transitions, which are not supported", id,
			)
		}
		offset, set = secs, true
		if name == "" {
			for n := range rule.Names {
				name = n
				break
			}
		}
	}
	if name == "" {
		name = string(id)
	}
	return time.FixedZone(name, offset), nil
}

// parseUTCOffset parses a JSCalendar UTCOffset ("+HH:MM" or "-HH:MM", optionally
// with seconds) into a signed second count (RFC 8984, Section 1.4.8).
func parseUTCOffset(s string) (int, error) {
	if len(s) < 6 || (s[0] != '+' && s[0] != '-') {
		return 0, fmt.Errorf("invalid UTC offset %q", s)
	}
	sign := 1
	if s[0] == '-' {
		sign = -1
	}
	body := s[1:]
	if body[2] != ':' {
		return 0, fmt.Errorf("invalid UTC offset %q", s)
	}
	hh, ok1 := parseTwo(body[0:2])
	mm, ok2 := parseTwo(body[3:5])
	if !ok1 || !ok2 {
		return 0, fmt.Errorf("invalid UTC offset %q", s)
	}
	ss := 0
	if len(body) >= 8 && body[5] == ':' {
		v, ok := parseTwo(body[6:8])
		if !ok {
			return 0, fmt.Errorf("invalid UTC offset %q", s)
		}
		ss = v
	}
	return sign * (hh*3600 + mm*60 + ss), nil
}

// parseTwo parses a two-digit field, reporting ok=false for non-digits.
func parseTwo(s string) (int, bool) {
	if len(s) != 2 || s[0] < '0' || s[0] > '9' || s[1] < '0' || s[1] > '9' {
		return 0, false
	}
	return int(s[0]-'0')*10 + int(s[1]-'0'), true
}

// localToTime places a floating [jscalendar.LocalDateTime] at the matching
// instant in loc.
func localToTime(lt jscalendar.LocalDateTime, loc *time.Location) time.Time {
	return time.Date(lt.Year, time.Month(lt.Month), lt.Day, lt.Hour, lt.Minute, lt.Second, 0, loc)
}

// timeToLocal reads the wall-clock fields of t back into a floating
// [jscalendar.LocalDateTime], discarding the zone — the recurrence id is a
// wall-clock value in the master's zone, not an absolute instant.
func timeToLocal(t time.Time) jscalendar.LocalDateTime {
	return jscalendar.LocalDateTime{
		Year:   t.Year(),
		Month:  int(t.Month()),
		Day:    t.Day(),
		Hour:   t.Hour(),
		Minute: t.Minute(),
		Second: t.Second(),
	}
}

// inWindow reports whether t lies in the half-open window [from, until):
// at or after from and strictly before until.
func inWindow(t, from, until time.Time) bool {
	return !t.Before(from) && t.Before(until)
}

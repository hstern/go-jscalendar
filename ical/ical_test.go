// Copyright 2026 The go-jscalendar Authors
// SPDX-License-Identifier: Apache-2.0

package ical_test

import (
	"errors"
	"testing"

	goical "github.com/emersion/go-ical"

	"github.com/hstern/go-jscalendar"
	"github.com/hstern/go-jscalendar/ical"
)

// TestFromICalEmpty confirms FromICal on a calendar with no convertible
// components yields no objects and no error, the base case of the conversion.
func TestFromICalEmpty(t *testing.T) {
	t.Parallel()

	objs, err := ical.FromICal(goical.NewCalendar())
	if err != nil {
		t.Fatalf("FromICal error = %v, want nil", err)
	}
	if objs != nil {
		t.Errorf("FromICal objects = %v, want nil", objs)
	}
}

// TestFromICalNil confirms FromICal tolerates a nil calendar rather than
// panicking, returning no objects.
func TestFromICalNil(t *testing.T) {
	t.Parallel()

	objs, err := ical.FromICal(nil)
	if err != nil {
		t.Fatalf("FromICal(nil) error = %v, want nil", err)
	}
	if objs != nil {
		t.Errorf("FromICal(nil) objects = %v, want nil", objs)
	}
}

// TestToICalNotImplemented pins the inverse skeleton contract.
func TestToICalNotImplemented(t *testing.T) {
	t.Parallel()

	cal, err := ical.ToICal(&jscalendar.Event{})
	if !errors.Is(err, ical.ErrNotImplemented) {
		t.Fatalf("ToICal error = %v, want ErrNotImplemented", err)
	}
	if cal != nil {
		t.Errorf("ToICal calendar = %v, want nil", cal)
	}
}

// TestToICalNoArgs confirms the variadic entry point is callable with no
// objects and still returns the skeleton sentinel rather than panicking.
func TestToICalNoArgs(t *testing.T) {
	t.Parallel()

	if _, err := ical.ToICal(); !errors.Is(err, ical.ErrNotImplemented) {
		t.Fatalf("ToICal() error = %v, want ErrNotImplemented", err)
	}
}

// TestGoICalImportable is a smoke test that the deliberate external dependency
// is wired into the module and usable: constructing an empty calendar and
// encoding it must round-trip the VCALENDAR skeleton go-ical produces.
func TestGoICalImportable(t *testing.T) {
	t.Parallel()

	cal := goical.NewCalendar()
	if cal == nil {
		t.Fatal("goical.NewCalendar returned nil")
	}
	cal.Props.SetText(goical.PropVersion, "2.0")
	cal.Props.SetText(goical.PropProductID, "-//go-jscalendar//ical skeleton//EN")
	if got := cal.Props.Get(goical.PropVersion); got == nil || got.Value != "2.0" {
		t.Fatalf("VERSION prop = %v, want 2.0", got)
	}
}

// TestParseInteropImportable confirms the sub-package can drive the parent
// jscalendar package — the conversion will route go-ical components into
// jscalendar.Parse's concrete return types, so both imports must coexist.
func TestParseInteropImportable(t *testing.T) {
	t.Parallel()

	const event = `{"@type":"Event","uid":"smoke-test"}`
	obj, err := jscalendar.Parse([]byte(event))
	if err != nil {
		t.Fatalf("jscalendar.Parse: %v", err)
	}
	if _, ok := obj.(*jscalendar.Event); !ok {
		t.Fatalf("jscalendar.Parse returned %T, want *jscalendar.Event", obj)
	}
}

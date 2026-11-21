package moon

import (
	"math"
	"testing"
	"time"
)

func mustDate(t *testing.T, date string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.DateOnly, date)
	if err != nil {
		t.Fatalf("fixture date %q does not parse: %v", date, err)
	}
	return parsed
}

// Anchor values below are the US Naval Observatory's published
// fractions of the Moon's disk illuminated at 0h UT, from the
// "Fraction of the Moon Illuminated" table,
// https://aa.usno.navy.mil/data/MoonFraction (USNO publishes the
// fraction rounded to two decimals). The dates are picked mid-phase,
// away from the sector boundaries where the phase name changes, and
// together cover all eight classical phase names:
//
//	date         phase            USNO
//	2000-01-06   New Moon         0.01
//	2000-01-14   First Quarter    0.44
//	2000-01-21   Full Moon        1.00
//	2001-06-21   New Moon         0.00
//	2010-12-18   Waxing Gibbous   0.88
//	2011-07-05   Waxing Crescent  0.16
//	2011-12-15   Waning Gibbous   0.81
//	2011-12-18   Last Quarter     0.51
//	2011-12-22   Waning Crescent  0.10
//
// The New Moon and Full Moon anchors are corroborated by the eclipse
// instants in TestKnownEclipseInstants (a solar eclipse is a new
// moon, a lunar eclipse a full moon).
func TestIlluminationMatchesUSNOPublishedFractions(t *testing.T) {
	anchors := []struct {
		date  string
		phase string
		usno  float64
	}{
		{date: "2000-01-06", phase: NewMoon, usno: 0.01},
		{date: "2000-01-14", phase: FirstQuarter, usno: 0.44},
		{date: "2000-01-21", phase: FullMoon, usno: 1.00},
		{date: "2001-06-21", phase: NewMoon, usno: 0.00},
		{date: "2010-12-18", phase: WaxingGibbous, usno: 0.88},
		{date: "2011-07-05", phase: WaxingCrescent, usno: 0.16},
		{date: "2011-12-15", phase: WaningGibbous, usno: 0.81},
		{date: "2011-12-18", phase: LastQuarter, usno: 0.51},
		{date: "2011-12-22", phase: WaningCrescent, usno: 0.10},
	}
	for _, a := range anchors {
		t.Run(a.date, func(t *testing.T) {
			info := For(mustDate(t, a.date))
			if info.Date != a.date {
				t.Fatalf("Info.Date = %q, want %q", info.Date, a.date)
			}
			if info.Phase != a.phase {
				t.Fatalf("For(%q).Phase = %q, want %q", a.date, info.Phase, a.phase)
			}
			if diff := math.Abs(info.Illumination - a.usno); diff > 0.01+1e-9 {
				t.Fatalf("For(%q).Illumination = %.4f, USNO publishes %.2f (difference %.4f exceeds 0.01)",
					a.date, info.Illumination, a.usno, diff)
			}
		})
	}
}

// Known eclipse instants (dates per NASA's eclipse bulletins) pin the
// extremes of the cycle: at a solar eclipse the Moon is new, at a
// lunar eclipse it is full. The engine must agree at those instants
// before its fractions can be compared with USNO's table above.
func TestKnownEclipseInstants(t *testing.T) {
	instants := []struct {
		when     string
		phase    string
		min, max float64
		note     string
	}{
		{"2001-06-21T12:03:41Z", NewMoon, 0.0, 0.005, "total solar eclipse"},
		{"2012-11-13T22:11:48Z", NewMoon, 0.0, 0.005, "total solar eclipse"},
		{"2011-06-15T20:12:36Z", FullMoon, 0.995, 1.0, "total lunar eclipse"},
		{"2015-09-28T02:47:07Z", FullMoon, 0.995, 1.0, "total lunar eclipse"},
	}
	for _, i := range instants {
		t.Run(i.when, func(t *testing.T) {
			when, err := time.Parse(time.RFC3339, i.when)
			if err != nil {
				t.Fatalf("fixture instant %q does not parse: %v", i.when, err)
			}
			illumination, cycleAngle := stateAt(when)
			if illumination < i.min || illumination > i.max {
				t.Fatalf("illumination at %s (%s) = %.5f, want within [%v, %v]",
					i.when, i.note, illumination, i.min, i.max)
			}
			if phase := phaseName(cycleAngle); phase != i.phase {
				t.Fatalf("phase at %s (%s) = %q, want %q", i.when, i.note, phase, i.phase)
			}
		})
	}
}

// TestIlluminationStaysWithinUnitInterval sweeps more than a
// decade of dates checking the two invariants the API promises: the
// fraction stays in [0, 1] and the phase is always one of the eight
// classical names.
func TestIlluminationStaysWithinUnitInterval(t *testing.T) {
	valid := make(map[string]bool, len(phaseNames))
	for _, name := range phaseNames {
		valid[name] = true
	}
	for d := mustDate(t, "1999-01-01"); !d.After(mustDate(t, "2026-12-31")); d = d.AddDate(0, 0, 1) {
		info := For(d)
		if info.Illumination < 0 || info.Illumination > 1 {
			t.Fatalf("For(%s).Illumination = %v, outside [0, 1]", info.Date, info.Illumination)
		}
		if !valid[info.Phase] {
			t.Fatalf("For(%s).Phase = %q, not one of the eight classical names", info.Date, info.Phase)
		}
	}
}

// Over one synodic month starting inside the New Moon sector, the
// phase names must appear in cycle order, each exactly once.
func TestPhaseNamesCycleInOrder(t *testing.T) {
	want := []string{
		NewMoon, WaxingCrescent, FirstQuarter, WaxingGibbous,
		FullMoon, WaningGibbous, LastQuarter, WaningCrescent,
	}

	var got []string
	for d := mustDate(t, "2000-01-05"); !d.After(mustDate(t, "2000-02-03")); d = d.AddDate(0, 0, 1) {
		name := For(d).Phase
		if len(got) == 0 || got[len(got)-1] != name {
			got = append(got, name)
		}
	}

	if len(got) != len(want) {
		t.Fatalf("one synodic month produced %d phase names (%v), want %d", len(got), got, len(want))
	}
	for i, name := range got {
		if name != want[i] {
			t.Fatalf("phase name %d of the cycle = %q, want %q (sequence: %v)", i, name, want[i], got)
		}
	}
}

// Every instant of a UTC date answers for that date's 00:00 UTC, so
// the result never depends on the time of day the question was
// asked.
func TestForEvaluatesAtMidnightUTC(t *testing.T) {
	want := For(mustDate(t, "2011-07-05"))
	if want.Date != "2011-07-05" {
		t.Fatalf("Info.Date = %q, want %q", want.Date, "2011-07-05")
	}
	instants := []string{
		"2011-07-05T00:00:00Z",
		"2011-07-05T00:00:01Z",
		"2011-07-05T09:26:40Z",
		"2011-07-05T23:59:59Z",
		"2011-07-05T05:30:00+05:30", // 00:00 UTC on the same date
		"2011-07-04T19:00:00-05:00", // 00:00 UTC on the same date
	}
	for _, s := range instants {
		instant, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatalf("fixture instant %q does not parse: %v", s, err)
		}
		if got := For(instant); got != want {
			t.Fatalf("For(%s) = %+v, want %+v (all instants of a date share its 00:00 UTC answer)", s, got, want)
		}
	}
}

// The sector boundary rule: each name owns the ±22.5° band around
// its exact elongation, and an angle exactly on a band edge belongs
// to the following sector.
func TestPhaseNameSectorBoundaries(t *testing.T) {
	tests := []struct {
		cycleAngle float64
		want       string
	}{
		{0, NewMoon},
		{22.4, NewMoon},
		{22.5, WaxingCrescent},
		{45, WaxingCrescent},
		{67.4, WaxingCrescent},
		{67.5, FirstQuarter},
		{90, FirstQuarter},
		{112.5, WaxingGibbous},
		{135, WaxingGibbous},
		{157.5, FullMoon},
		{180, FullMoon},
		{202.5, WaningGibbous},
		{225, WaningGibbous},
		{247.5, LastQuarter},
		{270, LastQuarter},
		{292.5, WaningCrescent},
		{315, WaningCrescent},
		{337.5, NewMoon},
		{359.9, NewMoon},
	}
	for _, tt := range tests {
		if got := phaseName(tt.cycleAngle); got != tt.want {
			t.Fatalf("phaseName(%.1f°) = %q, want %q", tt.cycleAngle, got, tt.want)
		}
	}
}

// Package moon computes, for a UTC calendar date, the illuminated
// fraction of the Moon's disk and which of the eight classical phase
// names the date carries.
//
// This package is the sol-lib half of the moon feature ("do the math
// in the library"). It is the in-tree implementation of the
// github.com/zaephor-bot/sol-lib engine (sol-lib#1) until that module
// is published, at which point this package becomes a thin re-export
// of it; the service (internal/server) holds no astronomy of its own
// and consumes only the API below, so the swap is an import change,
// not a redesign.
//
// Conventions:
//
//   - A date's values are computed for 00:00 UTC, the instant the US
//     Naval Observatory publishes in its "Fraction of the Moon
//     Illuminated" table (https://aa.usno.navy.mil/data/MoonFraction).
//   - Illumination is the geocentric fraction of the disk that is
//     lit, a number in [0, 1]: 0 at new moon, 1 at full moon.
//   - The four principal phases are instants, not intervals, so each
//     of the eight names is assigned to the 45° share of the synodic
//     cycle (roughly 3.7 days) centred on its exact elongation: New
//     Moon 0°, Waxing Crescent 45°, First Quarter 90°, Waxing
//     Gibbous 135°, Full Moon 180°, Waning Gibbous 225°, Last
//     Quarter 270°, Waning Crescent 315°. A cycle angle that lands
//     exactly on a sector boundary (22.5° + n·45°) belongs to the
//     following sector.
//
// Accuracy: positions follow Meeus, "Astronomical Algorithms" (2nd
// ed.), chapters 25 (Sun), 47 (Moon) and 48 (phase angle); see
// ephemeris.go. Against USNO's published two-decimal fractions the
// results agree well within 0.01 from the 18th through the 22nd
// century.
package moon

import (
	"math"
	"time"
)

// The eight classical phase names, in cycle order starting at new
// moon.
const (
	NewMoon        = "New Moon"
	WaxingCrescent = "Waxing Crescent"
	FirstQuarter   = "First Quarter"
	WaxingGibbous  = "Waxing Gibbous"
	FullMoon       = "Full Moon"
	WaningGibbous  = "Waning Gibbous"
	LastQuarter    = "Last Quarter"
	WaningCrescent = "Waning Crescent"
)

// phaseNames lists the names in cycle order; index n is the sector
// centred on elongation n·45°.
var phaseNames = [...]string{
	NewMoon,
	WaxingCrescent,
	FirstQuarter,
	WaxingGibbous,
	FullMoon,
	WaningGibbous,
	LastQuarter,
	WaningCrescent,
}

// Info is the library's answer for one UTC date. The field names are
// the service's public JSON contract and will not be renamed.
type Info struct {
	// Date is the UTC calendar date the values were computed for, in
	// YYYY-MM-DD form.
	Date string `json:"date"`

	// Illumination is the fraction of the lunar disk lit at 00:00 UTC
	// of Date, between 0 (new moon) and 1 (full moon). It is returned
	// raw; USNO publishes the same quantity rounded to two decimals.
	Illumination float64 `json:"illumination"`

	// Phase is one of the eight classical phase names (see the
	// package comment for how a date is assigned its name).
	Phase string `json:"phase"`
}

// For returns the Info for the UTC calendar date of t: every instant
// of a date answers for that date's 00:00 UTC, the instant the USNO
// table publishes, so a date's answer never depends on the time of
// day the question was asked.
func For(t time.Time) Info {
	utc := t.UTC()
	midnight := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	illumination, cycleAngle := stateAt(midnight)
	return Info{
		Date:         midnight.Format(time.DateOnly),
		Illumination: illumination,
		Phase:        phaseName(cycleAngle),
	}
}

// phaseName assigns the classical phase name for a cycle angle (the
// Moon's elongation in ecliptic longitude, 0° at new moon, growing
// eastward through the cycle). Each name owns the ±22.5° band around
// its exact elongation; an angle exactly on a band edge rounds into
// the following sector.
func phaseName(cycleAngle float64) string {
	sector := int(math.Round(cycleAngle/45)) % len(phaseNames)
	if sector < 0 {
		sector += len(phaseNames)
	}
	return phaseNames[sector]
}

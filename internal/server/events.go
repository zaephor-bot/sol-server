package server

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/zaephor-bot/sol-lib/solar"
	"github.com/zaephor-bot/sol-lib/windows"
)

// The response's time fields are UTC with the offset stated as
// +00:00. Resolving an arbitrary lat/lon to a civil time zone needs a
// spatial time-zone database, which is out of scope for sol-server,
// so the service takes the plain-UTC answer and says so in every
// response (and in the README) rather than guessing a zone.
const (
	responseTimezone = "UTC"
	responseOffset   = "+00:00"
)

// handleEvents serves GET /v1/events?lat=&lon=&date=.
//
// The response is the library's answer for one place and one day:
//
//	{
//	  "latitude": 33.6,
//	  "longitude": -117.8,
//	  "date": "2026-06-21",
//	  "timezone": "UTC",
//	  "utc_offset": "+00:00",
//	  "sunrise": "2026-06-21T12:41:23Z",
//	  "sunset": "2026-06-22T03:04:37Z",
//	  "solar_noon": "2026-06-21T19:53:12Z",
//	  "golden_hour": {
//	    "morning": {"start": "…", "end": "…"},
//	    "evening": {"start": "…", "end": "…"}
//	  },
//	  "blue_hour": {
//	    "dawn": {"start": "…", "end": "…"},
//	    "dusk": {"start": "…", "end": "…"}
//	  },
//	  "civil_twilight": {"start": "…", "end": "…"}
//	}
//
// Contract (the keys are frozen and will not be renamed):
//
//   - lat, lon and date are required query parameters. lat is a
//     decimal latitude in degrees within [-90, 90]; lon is a decimal
//     longitude in degrees within [-180, 180]; date is a strict
//     YYYY-MM-DD calendar day. A request missing any of them, or
//     carrying a malformed or out-of-range value, is answered with
//     400 and a human-readable error. date is never defaulted to
//     today, so answers stay deterministic.
//   - Every time is an RFC 3339 instant in UTC (see the constants
//     above); timezone and utc_offset state that once for the whole
//     response.
//   - sunrise, sunset and solar_noon come from the solar package;
//     the windows come from the windows package. The day is the UTC
//     calendar day of date, the same day every field describes.
//   - A day can hold some events but not all — a midnight sun still
//     has a noon, a polar-night day can still have a blue hour. A
//     missing event is null, never a bogus time, and the response
//     stays 200.
//   - civil_twilight is the classic begin/end pair: start is the
//     sun's -6° rising crossing that opens morning civil twilight,
//     end is the -6° falling crossing that closes evening civil
//     twilight.
//   - A day where nothing happens at all — no sunrise, no sunset,
//     and not one golden-hour, blue-hour or civil-twilight window,
//     the polar summer or polar winter — is answered with 422 and a
//     message saying there are no sun events that day. Solar noon
//     happens on every day, so it alone does not keep a day off the
//     422 path.
//
// The endpoint holds no astronomy of its own: it parses and
// range-checks the input, asks sol-lib for the answers, and writes
// back what the library returned.
func handleEvents(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	latitude, err := parseLatitude(query.Get("lat"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	longitude, err := parseLongitude(query.Get("lon"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	rawDate := query.Get("date")
	if rawDate == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{
			Error: `missing required query parameter "date": pass a UTC calendar date in YYYY-MM-DD form, e.g. ?date=2026-06-21`,
		})
		return
	}
	day, err := parseCalendarDate(rawDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}

	response, err := newEventsResponse(day, latitude, longitude)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{
			Error: fmt.Sprintf("computing sun events for lat=%v, lon=%v, date=%s: %v",
				latitude, longitude, rawDate, err),
		})
		return
	}
	if !response.anyEvent() {
		writeJSON(w, http.StatusUnprocessableEntity, errorBody{
			Error: fmt.Sprintf("there are no sun events at latitude %v, longitude %v on %s: "+
				"the sun neither rises nor sets and no golden-hour, blue-hour or "+
				"civil-twilight window opens (polar day or polar night)",
				latitude, longitude, rawDate),
		})
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// eventsResponse is the /v1/events answer for one place and day. The
// field names are the endpoint's public JSON contract and will not be
// renamed.
type eventsResponse struct {
	// Latitude and Longitude echo the request's place, in decimal
	// degrees (north and east positive). Date echoes the request's
	// calendar day in YYYY-MM-DD form.
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Date      string  `json:"date"`

	// Timezone and UTCOffset state the zone every time in the
	// response is expressed in (see the package constants).
	Timezone  string `json:"timezone"`
	UTCOffset string `json:"utc_offset"`

	// Sunrise, Sunset and SolarNoon are the day's horizon events;
	// SolarNoon is the day's transit, which happens even when the
	// sun never rises or sets. A missing event is null.
	Sunrise   *string `json:"sunrise"`
	Sunset    *string `json:"sunset"`
	SolarNoon *string `json:"solar_noon"`

	// GoldenHour carries the morning and evening golden-hour
	// windows; BlueHour the dawn and dusk blue-hour windows. An
	// absent window is null.
	GoldenHour goldenHourWindows `json:"golden_hour"`
	BlueHour   blueHourWindows   `json:"blue_hour"`

	// CivilTwilight is the classic begin/end pair: Start opens
	// morning civil twilight (the sun's -6° rising crossing), End
	// closes evening civil twilight (the -6° falling crossing). A
	// missing edge is null.
	CivilTwilight civilTwilightSpan `json:"civil_twilight"`
}

// goldenHourWindows holds the day's two golden hours.
type goldenHourWindows struct {
	Morning *eventWindow `json:"morning"` // horizon to +6° while rising
	Evening *eventWindow `json:"evening"` // +6° back to horizon while setting
}

// blueHourWindows holds the day's two blue hours.
type blueHourWindows struct {
	Dawn *eventWindow `json:"dawn"` // -6° to -4° while rising
	Dusk *eventWindow `json:"dusk"` // -4° to -6° while setting
}

// civilTwilightSpan is the day's civil twilight as a begin/end pair.
type civilTwilightSpan struct {
	Start *string `json:"start"` // -6° rising crossing (morning)
	End   *string `json:"end"`   // -6° falling crossing (evening)
}

// eventWindow is one timed window: when it opens and when it closes.
type eventWindow struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// newEventsResponse asks sol-lib for the day's events and windows and
// assembles the response, mapping every event that does not happen to
// nil. It returns an error only for a library failure it does not
// know how to interpret; the polar day and night are not failures.
func newEventsResponse(day time.Time, latitude, longitude float64) (eventsResponse, error) {
	response := eventsResponse{
		Latitude:  latitude,
		Longitude: longitude,
		Date:      day.Format(time.DateOnly),
		Timezone:  responseTimezone,
		UTCOffset: responseOffset,
	}

	if sunrise, err := solar.Sunrise(day, latitude, longitude); err == nil {
		response.Sunrise = formatEvent(sunrise)
	} else if !absentEvent(err) {
		return eventsResponse{}, err
	}
	if sunset, err := solar.Sunset(day, latitude, longitude); err == nil {
		response.Sunset = formatEvent(sunset)
	} else if !absentEvent(err) {
		return eventsResponse{}, err
	}
	if noon, err := solar.SolarNoon(day, latitude, longitude); err == nil {
		response.SolarNoon = formatEvent(noon)
	} else {
		// SolarNoon's contract says the error is always nil; treat
		// any deviation as a failure rather than a missing event.
		return eventsResponse{}, err
	}

	for _, window := range windows.All(day, latitude, longitude) {
		switch window.Event {
		case windows.DawnGoldenHour:
			response.GoldenHour.Morning = presentWindow(window)
		case windows.DuskGoldenHour:
			response.GoldenHour.Evening = presentWindow(window)
		case windows.DawnBlueHour:
			response.BlueHour.Dawn = presentWindow(window)
		case windows.DuskBlueHour:
			response.BlueHour.Dusk = presentWindow(window)
		case windows.DawnCivilTwilight:
			if !window.Absent {
				response.CivilTwilight.Start = formatEvent(window.Start)
			}
		case windows.DuskCivilTwilight:
			if !window.Absent {
				response.CivilTwilight.End = formatEvent(window.End)
			}
		}
	}
	return response, nil
}

// anyEvent reports whether at least one horizon or twilight event
// happens on the day. Solar noon alone does not count: it happens on
// every day, so a day whose only "event" is the transit is a day
// with no sun events at all (see the handler contract).
func (r eventsResponse) anyEvent() bool {
	return r.Sunrise != nil || r.Sunset != nil ||
		r.GoldenHour.Morning != nil || r.GoldenHour.Evening != nil ||
		r.BlueHour.Dawn != nil || r.BlueHour.Dusk != nil ||
		r.CivilTwilight.Start != nil || r.CivilTwilight.End != nil
}

// presentWindow converts a library window into a response window,
// mapping an absent window to nil (JSON null).
func presentWindow(window windows.Window) *eventWindow {
	if window.Absent {
		return nil
	}
	return &eventWindow{
		Start: window.Start.Format(time.RFC3339),
		End:   window.End.Format(time.RFC3339),
	}
}

// formatEvent renders an event instant as an RFC 3339 string in the
// response's stated zone (UTC).
func formatEvent(t time.Time) *string {
	formatted := t.Format(time.RFC3339)
	return &formatted
}

// absentEvent reports whether err is one of sol-lib's "the event does
// not happen" sentinels rather than a failure.
func absentEvent(err error) bool {
	return errors.Is(err, solar.ErrSunNeverRises) || errors.Is(err, solar.ErrSunNeverSets)
}

// parseLatitude parses the required lat query parameter: a decimal
// latitude in degrees within [-90, 90], positive north.
func parseLatitude(raw string) (float64, error) {
	if raw == "" {
		return 0, fmt.Errorf(`missing required query parameter "lat": pass a latitude in decimal degrees, e.g. ?lat=33.6`)
	}
	latitude, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(latitude) || math.IsInf(latitude, 0) {
		return 0, fmt.Errorf("invalid lat %q: want a latitude in decimal degrees between -90 and 90, e.g. 33.6", raw)
	}
	if latitude < -90 || latitude > 90 {
		return 0, fmt.Errorf("lat %v is out of range: latitude must lie between -90 and 90 degrees", latitude)
	}
	return latitude, nil
}

// parseLongitude parses the required lon query parameter: a decimal
// longitude in degrees within [-180, 180], positive east.
func parseLongitude(raw string) (float64, error) {
	if raw == "" {
		return 0, fmt.Errorf(`missing required query parameter "lon": pass a longitude in decimal degrees, e.g. ?lon=-117.8`)
	}
	longitude, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(longitude) || math.IsInf(longitude, 0) {
		return 0, fmt.Errorf("invalid lon %q: want a longitude in decimal degrees between -180 and 180, e.g. -117.8", raw)
	}
	if longitude < -180 || longitude > 180 {
		return 0, fmt.Errorf("lon %v is out of range: longitude must lie between -180 and 180 degrees", longitude)
	}
	return longitude, nil
}

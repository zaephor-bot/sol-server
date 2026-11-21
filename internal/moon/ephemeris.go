package moon

import (
	"math"
	"time"
)

// The ephemeris behind For, after Meeus, "Astronomical Algorithms"
// (2nd edition, Willmann-Bell, 1998):
//
//   - chapter 25: the Sun's apparent geocentric ecliptic longitude
//     and distance (the "low precision" series, ±0.01° — ample here,
//     since only the Sun–Moon elongation matters);
//   - chapter 47: the Moon's geocentric ecliptic longitude, latitude
//     and distance (the truncated ELP-2000/82 series);
//   - chapter 48: the phase angle and the illuminated fraction.
//
// Instants are interpreted as UT; the TT−UT offset (ΔT, under two
// minutes in the era of interest) shifts the Moon's longitude by at
// most ~0.01° and the illuminated fraction by less than 0.0001.

const degToRad = math.Pi / 180

// auKm is one astronomical unit in kilometres (Meeus, p. 407).
const auKm = 149597870.7

func sind(deg float64) float64 { return math.Sin(deg * degToRad) }
func cosd(deg float64) float64 { return math.Cos(deg * degToRad) }

// norm360 reduces an angle to [0°, 360°).
func norm360(deg float64) float64 {
	deg = math.Mod(deg, 360)
	if deg < 0 {
		deg += 360
	}
	return deg
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// julianCenturies returns T, Julian centuries elapsed since the
// epoch J2000.0 (JD 2451545.0) at the given instant.
func julianCenturies(t time.Time) float64 {
	jd := float64(t.Unix())/86400 + 2440587.5
	return (jd - 2451545.0) / 36525
}

// sunEcliptic returns the Sun's apparent geocentric ecliptic
// longitude in degrees and its distance from Earth in kilometres
// (Meeus chapter 25).
func sunEcliptic(T float64) (lambda, distanceKm float64) {
	meanLongitude := 280.46646 + 36000.76983*T + 0.0003032*T*T
	meanAnomaly := 357.52911 + 35999.05029*T - 0.0001537*T*T
	eccentricity := 0.016708634 - 0.000042037*T - 0.0000001267*T*T

	equationOfCenter := (1.914602-0.004817*T-0.000014*T*T)*sind(meanAnomaly) +
		(0.019993-0.000101*T)*sind(2*meanAnomaly) +
		0.000289*sind(3*meanAnomaly)

	trueLongitude := meanLongitude + equationOfCenter
	trueAnomaly := meanAnomaly + equationOfCenter

	// Apparent longitude: aberration plus nutation in longitude.
	omega := 125.04 - 1934.136*T
	lambda = trueLongitude - 0.00569 - 0.00478*sind(omega)

	distanceAU := 1.000001018 * (1 - eccentricity*eccentricity) /
		(1 + eccentricity*cosd(trueAnomaly))
	distanceKm = distanceAU * auKm
	return lambda, distanceKm
}

// moonTerm is one row of Meeus table 47.A. d, m, mPrime and f are
// the multipliers of the arguments D, M, M', F; longitude is the Σl
// coefficient in millionths of a degree, distance the Σr coefficient
// in thousandths of a kilometre.
type moonTerm struct {
	d, m, mPrime, f int
	longitude       float64
	distance        float64
}

// moonTerms carries the terms of Meeus table 47.A large enough to
// place the Moon's longitude within ~0.05°; the omitted terms are
// individually below 0.002° and shift the illuminated fraction by
// less than 0.0005.
var moonTerms = []moonTerm{
	{0, 0, 1, 0, 6288774, -20905355},
	{2, 0, -1, 0, 1274027, -3699111},
	{2, 0, 0, 0, 658314, -2955968},
	{0, 0, 2, 0, 213618, -569925},
	{0, 1, 0, 0, -185116, 48888},
	{0, 0, 0, 2, -114332, -3149},
	{2, 0, -2, 0, 58793, 246158},
	{2, -1, -1, 0, 57066, -152138},
	{2, 0, 1, 0, 53322, -170733},
	{2, -1, 0, 0, 45758, -204586},
	{0, 1, -1, 0, -40923, -129620},
	{1, 0, 0, 0, -34720, 108743},
	{0, 1, 1, 0, -30383, 104755},
	{2, 0, 0, -2, 15327, 10321},
	{0, 0, 1, 2, -12528, 0},
	{0, 0, 1, -2, 10980, 79661},
	{4, 0, -1, 0, 10675, -34782},
	{0, 0, 3, 0, 10034, -23210},
	{4, 0, -2, 0, 8548, -21636},
	{2, 1, -1, 0, -7888, 24208},
	{2, 1, 0, 0, -6766, 30824},
	{1, 0, -1, 0, -5163, -8379},
	{1, 1, 0, 0, 4987, -16675},
	{2, -1, 1, 0, 4036, -12831},
	{2, 0, 2, 0, 3994, -10445},
	{4, 0, 0, 0, 3861, -11650},
	{2, 0, -3, 0, 3665, 14403},
	{0, 1, -2, 0, -2689, -7003},
	{2, 0, -1, 2, -2602, 0},
	{2, -1, -2, 0, 2390, 10056},
	{1, 0, 1, 0, -2348, 6322},
	{2, -2, 0, 0, 2236, -9884},
	{0, 1, 2, 0, -2120, 5751},
	{0, 2, 0, 0, -2069, 0},
	{2, -2, -1, 0, 2048, -4950},
	{2, 0, 1, -2, -1773, 4130},
}

// latitudeTerm is one row of Meeus table 47.B; latitude is the Σb
// coefficient in millionths of a degree.
type latitudeTerm struct {
	d, m, mPrime, f int
	latitude        float64
}

var latitudeTerms = []latitudeTerm{
	{0, 0, 0, 1, 5128122},
	{0, 0, 1, 1, 280602},
	{0, 0, 1, -1, 277693},
	{2, 0, 0, -1, 173237},
	{2, 0, -1, 1, 55413},
	{2, 0, -1, -1, 46271},
	{2, 0, 0, 1, 32573},
	{0, 0, 2, 1, 17198},
	{2, 0, 1, -1, 9266},
	{0, 0, 2, -1, 8822},
	{2, -1, 0, -1, 8216},
	{2, 0, -2, -1, 4324},
	{2, 0, 1, 1, 4200},
	{2, 1, 0, -1, -3359},
	{2, -1, -1, 1, 2463},
	{2, -1, 0, 1, 2211},
	{2, -1, -1, -1, 2065},
	{0, 1, -1, -1, -1870},
	{4, 0, -1, -1, 1828},
	{0, 1, 0, 1, -1794},
	{0, 0, 0, 3, -1749},
	{0, 1, -1, 1, -1565},
	{1, 0, 0, 1, -1491},
	{0, 1, 1, 1, -1475},
	{0, 1, 1, -1, -1410},
	{0, 1, 0, -1, -1344},
	{1, 0, 0, -1, -1335},
}

// moonEcliptic returns the Moon's geocentric ecliptic longitude and
// latitude in degrees and its distance from Earth in kilometres
// (Meeus chapter 47).
func moonEcliptic(T float64) (lambda, beta, distanceKm float64) {
	// Fundamental arguments, Meeus 47.1–47.6.
	meanLongitude := 218.3164477 + 481267.88123421*T -
		0.0015786*T*T + T*T*T/538841 - T*T*T*T/65194000
	meanElongation := 297.8501921 + 445267.1114034*T -
		0.0018819*T*T + T*T*T/545868 - T*T*T*T/113065000
	sunMeanAnomaly := 357.5291092 + 35999.0502909*T -
		0.0001536*T*T + T*T*T/24490000
	moonMeanAnomaly := 134.9633964 + 477198.8675055*T +
		0.0087414*T*T + T*T*T/69699 - T*T*T*T/14712000
	argumentOfLatitude := 93.2720950 + 483202.0175233*T -
		0.0036539*T*T - T*T*T/3526000 + T*T*T*T/863310000

	// The eccentricity factor: terms involving M are damped as the
	// solar orbit's eccentricity evolves (Meeus, p. 342).
	eccentricity := 1 - 0.002516*T - 0.0000074*T*T
	damping := func(m int) float64 {
		return math.Pow(eccentricity, math.Abs(float64(m)))
	}

	// Additive arguments for the planetary and secular corrections
	// (Meeus 47.7–47.9 and p. 343).
	additive1 := 119.75 + 131.849*T
	additive2 := 53.09 + 479264.290*T
	additive3 := 313.45 + 481266.484*T

	var sumLongitude, sumDistance, sumLatitude float64
	for _, term := range moonTerms {
		arg := float64(term.d)*meanElongation +
			float64(term.m)*sunMeanAnomaly +
			float64(term.mPrime)*moonMeanAnomaly +
			float64(term.f)*argumentOfLatitude
		e := damping(term.m)
		sumLongitude += term.longitude * e * sind(arg)
		sumDistance += term.distance * e * cosd(arg)
	}
	for _, term := range latitudeTerms {
		arg := float64(term.d)*meanElongation +
			float64(term.m)*sunMeanAnomaly +
			float64(term.mPrime)*moonMeanAnomaly +
			float64(term.f)*argumentOfLatitude
		sumLatitude += term.latitude * damping(term.m) * sind(arg)
	}

	lambda = meanLongitude + sumLongitude/1e6 +
		(3958*sind(additive1)+1962*sind(meanLongitude-argumentOfLatitude)+
			318*sind(additive2))/1e6
	distanceKm = 385000.56 + sumDistance/1000
	beta = sumLatitude/1e6 +
		(-2235*sind(meanLongitude)+382*sind(additive3)+
			175*sind(additive1-argumentOfLatitude)-
			175*sind(additive1+argumentOfLatitude)-
			127*sind(meanLongitude-sunMeanAnomaly))/1e6
	return lambda, beta, distanceKm
}

// stateAt returns the illumination and cycle angle for an instant.
//
// illumination is the geocentric fraction of the lunar disk that is
// lit, (1 + cos i)/2 with i the phase angle (Meeus 48.1), clamped to
// [0, 1]: 0 at new moon, 1 at full moon.
//
// cycleAngle is the Moon's elongation from the Sun in ecliptic
// longitude, reduced to [0°, 360°): 0° at new moon, growing eastward
// — 90° at first quarter, 180° at full moon, 270° at last quarter.
func stateAt(t time.Time) (illumination, cycleAngle float64) {
	T := julianCenturies(t)
	sunLongitude, sunDistance := sunEcliptic(T)
	moonLongitude, moonBeta, moonDistance := moonEcliptic(T)

	cycleAngle = norm360(moonLongitude - sunLongitude)

	// Geocentric elongation ψ of the Moon from the Sun (Meeus 48.2):
	// only its cosine is needed.
	cosElongation := cosd(moonBeta) * cosd(cycleAngle)

	// Phase angle i (Meeus 48.3, rearranged into a cosine): with R
	// the Sun–Earth distance and Δ the Earth–Moon distance,
	//
	//	cos i = (Δ − R cos ψ) / √(Δ² + R² − 2ΔR cos ψ)
	//
	// which is −1 at new moon and +1 at full moon.
	cosPhaseAngle := (moonDistance - sunDistance*cosElongation) /
		math.Sqrt(moonDistance*moonDistance+sunDistance*sunDistance-
			2*moonDistance*sunDistance*cosElongation)

	illumination = (1 + clamp(cosPhaseAngle, -1, 1)) / 2
	return illumination, cycleAngle
}

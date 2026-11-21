# sol-server

A small HTTP service with environment-based port configuration and
graceful shutdown, packaged as a minimal container image that ships
just the binary. The endpoints wrap the shared solar library
[`github.com/zaephor-bot/sol-lib`](https://github.com/zaephor-bot/sol-lib)
(version pinned in `go.mod`): the service owns no astronomy of its
own, it turns library answers into HTTP responses.

## Quickstart

### Build

With Docker (multi-stage: the build stage compiles and checks the
service, the final stage contains only the static binary — no
compiler, source tree, or build tools):

```sh
docker build -t sol-server .
```

Native Go build:

```sh
go build -o sol-server .
```

### Run

The server listens on the port named by the `PORT` environment
variable, defaulting to `8080` when it is unset:

```sh
./sol-server                 # listens on :8080
PORT=9090 ./sol-server       # listens on :9090

docker run --rm -p 8080:8080 sol-server
docker run --rm -p 9090:9090 -e PORT=9090 sol-server
```

If `PORT` holds a junk value (non-numeric, zero, or outside 1–65535),
the server logs a warning and falls back to `8080`.

### Endpoints

| Method | Path         | Description                                     |
| ------ | ------------ | ----------------------------------------------- |
| `GET`  | `/`          | Service index: name plus the endpoint list      |
| `GET`  | `/healthz`   | Liveness probe                                  |
| `GET`  | `/readyz`    | Readiness probe                                 |
| `GET`  | `/v1/events` | The day's sun events for a place and date       |
| `GET`  | `/v1/moon`   | Moon illumination and phase for a UTC date      |

Example:

```sh
curl http://localhost:8080/
# {"endpoints":["/","/healthz","/readyz","/v1/events","/v1/moon"],"service":"sol-server"}
curl http://localhost:8080/healthz
# {"status":"ok"}
```

Unknown paths return `404` and non-`GET` methods return `405`.

## Sun events: `GET /v1/events`

    GET /v1/events?lat=<decimal degrees>&lon=<decimal degrees>&date=<YYYY-MM-DD>

All three parameters are required. `lat` must lie in `[-90, 90]`
(positive north), `lon` in `[-180, 180]` (positive east), and `date`
must be a strict `YYYY-MM-DD` calendar day — it never defaults to
today, so answers stay deterministic. A missing, malformed, or
out-of-range parameter is answered with `400` and a human-readable
message in a fixed error shape, `{"error": "…"}` (the shape of every
error this service sends).

```sh
curl 'http://localhost:8080/v1/events?lat=33.6&lon=-117.8&date=2026-06-21'
```

```json
{"latitude":33.6,"longitude":-117.8,"date":"2026-06-21","timezone":"UTC","utc_offset":"+00:00","sunrise":"2026-06-21T12:41:30Z","sunset":"2026-06-22T03:04:41Z","solar_noon":"2026-06-21T19:53:06Z","golden_hour":{"morning":{"start":"2026-06-21T12:39:53Z","end":"2026-06-21T13:17:30Z"},"evening":{"start":"2026-06-22T02:28:42Z","end":"2026-06-22T03:06:18Z"}},"blue_hour":{"dawn":{"start":"2026-06-21T12:12:09Z","end":"2026-06-21T12:23:24Z"},"dusk":{"start":"2026-06-22T03:22:47Z","end":"2026-06-22T03:34:02Z"}},"civil_twilight":{"start":"2026-06-21T12:12:09Z","end":"2026-06-22T03:34:02Z"}}
```

The JSON keys are a contract other code builds on and will not be
renamed:

- `sunrise`, `sunset`, `solar_noon` — the day's horizon events;
  `solar_noon` is the day's transit, which happens even when the sun
  never rises or sets.
- `golden_hour` — the `morning` and `evening` golden-hour windows,
  each a `{start, end}` pair (horizon to +6° while the sun rises;
  +6° back to the horizon while it sets).
- `blue_hour` — the `dawn` and `dusk` blue-hour windows (−6° to −4°
  on the way up; −4° to −6° on the way down).
- `civil_twilight` — the classic begin/end pair: `start` is the −6°
  rising crossing opening morning civil twilight, `end` the −6°
  falling crossing closing evening civil twilight.

### Time zones

Every time comes back in **UTC**, and the response says so itself:
`timezone` is `"UTC"` and `utc_offset` is `"+00:00"`, stated once for
the whole response. Answering in a place's local clock time would
require resolving an arbitrary lat/lon to a civil time zone — a
spatial time-zone database that is out of scope for this service — so
the service takes the plain-UTC answer with the offset stated as zero
rather than guessing a zone.

### Partial days and sunless days

A day can hold some events but not all — a midnight sun still has a
noon; a deep polar-night day can still have a blue hour. Such a day
answers `200` with `null` for every missing event, never a bogus
time.

A day where *nothing* happens at all — no sunrise, no sunset, and not
one golden-hour, blue-hour, or civil-twilight window, the polar
summer or polar winter at either pole — answers `422`:

```sh
curl -i 'http://localhost:8080/v1/events?lat=89&lon=0&date=2026-06-21'
# HTTP/1.1 422 Unprocessable Entity
# {"error":"there are no sun events at latitude 89, longitude 0 on 2026-06-21: …"}
```

(Solar noon happens on every day, so it alone does not keep a day off
the `422` path.)

### The golden test

The exact `200` body above for `lat=33.6`, `lon=-117.8`,
`date=2026-06-21` is pinned in
[`testdata/events-2026-06-21-lat33.6-lon-117.8.json`](testdata/events-2026-06-21-lat33.6-lon-117.8.json)
and compared byte for byte by `TestEventsGoldenFile`. Its whole job is
to fail loudly when a key is renamed or a value drifts — including a
bumped `sol-lib` version, which is why the library is pinned in
`go.mod`.

## Graceful shutdown

On `SIGTERM` or `SIGINT` the server stops accepting new connections,
lets every in-flight request finish with a normal response, and then
exits with status `0`. The drain is bounded by a 10-second deadline —
shorter than the usual orchestrator grace period (Kubernetes defaults
to 30s) — so the process exits on its own terms rather than being
`SIGKILL`ed mid-drain. A startup failure (for example, the port is
already in use) exits with status `1`.

# sol-server

A small HTTP service with environment-based port configuration and
graceful shutdown, packaged as a minimal container image that ships
just the binary.

## Quickstart

### Build

With Docker (multi-stage: the build stage compiles and checks the
service, the final stage contains only the static binary — no
compiler, source tree, or build tools):

```sh
docker build -t sol-server .
```

Native Go build — the repository does not carry a `go.mod`, so create
the module file locally first (the Docker build stage does the same):

```sh
go mod init github.com/zaephor-bot/sol-server
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

| Method | Path      | Description                                    |
| ------ | --------- | ---------------------------------------------- |
| `GET`  | `/`       | Service index: name plus the endpoint list     |
| `GET`  | `/healthz`| Liveness probe                                 |
| `GET`  | `/readyz` | Readiness probe                                |

Example:

```sh
curl http://localhost:8080/
# {"endpoints":["/","/healthz","/readyz"],"service":"sol-server"}
curl http://localhost:8080/healthz
# {"status":"ok"}
```

Unknown paths return `404` and non-`GET` methods return `405`.

### Graceful shutdown

On `SIGTERM` or `SIGINT` the server stops accepting new connections,
lets every in-flight request finish with a normal response, and then
exits with status `0`. The drain is bounded by a 10-second deadline —
shorter than the usual orchestrator grace period (Kubernetes defaults
to 30s) — so the process exits on its own terms rather than being
`SIGKILL`ed mid-drain. A startup failure (for example, the port is
already in use) exits with status `1`.

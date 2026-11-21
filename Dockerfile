# syntax=docker/dockerfile:1

# ---- build stage: compile the service and run its checks ----
# The module file is generated here rather than committed, so the image
# build is self-contained (see README for native builds). CGO_ENABLED=0
# produces a static binary for the distroless final stage.
FROM golang:1.23-alpine AS build

ENV CGO_ENABLED=0

WORKDIR /src

COPY . .

RUN test -f go.mod || go mod init github.com/zaephor-bot/sol-server
RUN go vet ./... && go test ./...
RUN go build -trimpath -ldflags="-s -w" -o /out/sol-server .

# ---- final stage: just the binary ----
# distroless/static ships no shell, package manager, compiler, or
# source tree — only the static binary is copied in.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/sol-server /sol-server

# 8080 is the default listen port; override at runtime with -e PORT=...
ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/sol-server"]

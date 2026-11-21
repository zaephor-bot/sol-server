// Package config resolves sol-server's runtime configuration from the
// environment.
package config

import (
	"fmt"
	"strconv"
	"strings"
)

// DefaultPort is the port the server listens on whenever PORT is unset
// (or set to an invalid value; callers decide the fallback policy).
const DefaultPort = 8080

// Port reports the TCP port the server should listen on, read from the
// PORT environment variable via getenv (os.Getenv in production).
//
// When PORT is unset (or blank) it returns DefaultPort. When PORT is
// set but invalid — non-numeric, zero, or outside the valid TCP range
// — it returns a descriptive error and the caller chooses whether to
// abort or fall back to DefaultPort.
func Port(getenv func(string) string) (int, error) {
	raw := strings.TrimSpace(getenv("PORT"))
	if raw == "" {
		return DefaultPort, nil
	}
	port, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("PORT %q is not a number", raw)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("PORT %d is outside the valid TCP port range 1-65535", port)
	}
	return port, nil
}

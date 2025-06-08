// cmd/ping/main.go
//
// Build with:
//   ./scripts/build.sh ./cmd/ping ping
//
// Intended for Docker HEALTHCHECK:
//   HEALTHCHECK CMD ["/ping"]

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// -----------------------------------------------------------------------------
// Constants
// -----------------------------------------------------------------------------
const (
	defaultPort          = 8080
	healthEndpoint       = "/healthz"
	expectedHealthStatus = "ok"
	requestTimeout       = 1 * time.Second

	// exit codes
	codeRequestFailed     = 2
	codeBadHTTPStatus     = 3
	codeDecodeError       = 4
	codeReportedUnhealthy = 5

	// log / error templates
	msgRequestFailed     = "request failed: %v"
	msgBadHTTPStatus     = "unexpected HTTP status %d"
	msgDecodeError       = "decode error: %v"
	msgReportedUnhealthy = "service reported unhealthy: %q"
	msgHealthy           = "service healthy on port %d"
)

// healthResp mirrors the optional JSON body { "status": "ok" }.
type healthResp struct {
	Status string `json:"status"`
}

func main() {
	port := detectPort()
	url := fmt.Sprintf("http://localhost:%d%s", port, healthEndpoint)

	client := &http.Client{Timeout: requestTimeout}

	resp, err := client.Get(url)
	if err != nil {
		fail(codeRequestFailed, msgRequestFailed, err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		fail(codeBadHTTPStatus, msgBadHTTPStatus, resp.StatusCode)
	}

	var h healthResp
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil && !errors.Is(err, io.EOF) {
		fail(codeDecodeError, msgDecodeError, err)
	}
	if h.Status != "" && h.Status != expectedHealthStatus {
		fail(codeReportedUnhealthy, msgReportedUnhealthy, h.Status)
	}

	log.Printf(msgHealthy, port)
}

// detectPort parses APP_PORT and falls back to defaultPort.
func detectPort() int {
	if v := os.Getenv("APP_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 && p <= 65535 {
			return p
		}
	}
	return defaultPort
}

// fail logs a message and exits with the given code.
func fail(code int, format string, args ...any) {
	log.Printf(format, args...)
	os.Exit(code)
}

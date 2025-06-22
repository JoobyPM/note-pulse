//go:build e2e

package test

import (
	"net"
	"os"
	"strconv"
	"testing"

	"note-pulse/internal/config"
)

// TestMain ensures config is reset for the first test in the package
// make sure environment for the very *first* test is clean
func TestMain(m *testing.M) {
	config.ResetCache()
	code := m.Run()
	os.Exit(code)
}

// randomPort asks the kernel for an unused TCP port.
func randomPort() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port), nil
}

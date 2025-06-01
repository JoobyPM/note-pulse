//go:build e2e

package test

import (
	"net"
	"strconv"
)

// randomPort asks the kernel for an unused TCP port.
func randomPort() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer l.Close()
	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port), nil
}

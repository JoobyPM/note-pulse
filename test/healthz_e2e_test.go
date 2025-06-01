//go:build e2e

package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"

	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestHealthzE2E(t *testing.T) {
	// global timeout for the whole test
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	//----------------------------------------------------------------------
	// MongoDB container
	//----------------------------------------------------------------------
	mongoC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "mongo:8.0",
			ExposedPorts: []string{"27017/tcp"},
			Env: map[string]string{
				"MONGO_INITDB_ROOT_USERNAME": "root",
				"MONGO_INITDB_ROOT_PASSWORD": "example",
				"MONGO_INITDB_DATABASE":      "e2e",
			},
			WaitingFor: wait.ForListeningPort("27017/tcp").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = mongoC.Terminate(context.Background()) })

	host, err := mongoC.Host(ctx)
	require.NoError(t, err)
	port, err := mongoC.MappedPort(ctx, "27017")
	require.NoError(t, err)
	mongoURI := fmt.Sprintf("mongodb://root:example@%s:%s/", host, port.Port())

	//----------------------------------------------------------------------
	// Start application
	//----------------------------------------------------------------------
	appPort, err := randomPort()
	require.NoError(t, err)

	// prepare /dev/null writers to avoid internal copy goroutines
	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	t.Cleanup(func() { _ = devNull.Close() })

	srvCtx, srvCancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(srvCtx, "go", "run", "./cmd/server")
	cmd.Dir = "../" // project root
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("MONGO_URI=%s", mongoURI),
		"MONGO_DB_NAME=e2e",
		"LOG_LEVEL=debug",
		fmt.Sprintf("APP_PORT=%s", appPort),
	)
	cmd.Stdout = devNull // no extra goroutines, no console spam
	cmd.Stderr = devNull
	require.NoError(t, cmd.Start())

	// ensure the process is gone before the test returns
	t.Cleanup(func() {
		srvCancel() // send SIGINT
		done := make(chan struct{})
		go func() {
			_ = cmd.Wait() // waits for pipes to drain
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	})

	//----------------------------------------------------------------------
	// Wait for health endpoint
	//----------------------------------------------------------------------
	healthURL := fmt.Sprintf("http://localhost:%s/api/v1/healthz", appPort)
	client := &http.Client{Timeout: 2 * time.Second}

	deadline := time.Now().Add(30 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("server never responded on %s", healthURL)
		}
		resp, err := client.Get(healthURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(200 * time.Millisecond)
	}

	//----------------------------------------------------------------------
	// Assertions
	//----------------------------------------------------------------------
	resp, err := client.Get(healthURL)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var payload map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
	assert.Equal(t, "ok", payload["status"])
}

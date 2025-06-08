//go:build e2e

package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"note-pulse/internal/config"
)

const (
	signUpEndpoint     = "/api/v1/auth/sign-up"
	refreshEndpoint    = "/api/v1/auth/refresh"
	signOutEndpoint    = "/api/v1/auth/sign-out"
	signInEndpoint     = "/api/v1/auth/sign-in"
	signOutAllEndpoint = "/api/v1/auth/sign-out-all"

	meEndpoint = "/api/v1/me"

	shouldRefreshedMsg = "should refresh"
)

// limitedWriter wraps an io.Writer and limits the amount of data written
type limitedWriter struct {
	w     io.Writer
	limit int64
	n     int64
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	if lw.n >= lw.limit {
		// Still drain the data so the pipe can't fill, but signal "written"
		lw.n += int64(len(p))
		return 0, io.ErrShortWrite
	}

	want := len(p)
	if remain := lw.limit - lw.n; int64(want) > remain {
		p = p[:remain]
	}

	n, err := lw.w.Write(p)
	lw.n += int64(n)
	if int64(want) > int64(n) && err == nil {
		err = io.ErrShortWrite
	}
	return n, err
}

// TestEnvironment holds the test infrastructure
type TestEnvironment struct {
	MongoContainer testcontainers.Container
	ServerCmd      *exec.Cmd
	ServerCancel   context.CancelFunc
	BaseURL        string
	Client         *http.Client
}

// startMongoTC returns a MongoDB testcontainer with (uri, terminateFn, error)
func startMongoTC(ctx context.Context, t *testing.T) (string, func(), error) {
	t.Helper()
	t.Log("Starting MongoDB container")
	mongoC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "mongo:8.0",
			ExposedPorts: []string{"27017/tcp"},
			Env: map[string]string{
				"MONGO_INITDB_ROOT_USERNAME": "root",
				"MONGO_INITDB_ROOT_PASSWORD": "example",
				"MONGO_INITDB_DATABASE":      "e2e",
			},
			WaitingFor: wait.ForExec([]string{"mongosh", "--eval", "db.adminCommand('ping')"}).
				WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		return "", nil, err
	}

	host, err := mongoC.Host(ctx)
	if err != nil {
		_ = mongoC.Terminate(ctx)
		return "", nil, err
	}

	port, err := mongoC.MappedPort(ctx, "27017")
	if err != nil {
		_ = mongoC.Terminate(ctx)
		return "", nil, err
	}

	mongoURI := fmt.Sprintf("mongodb://root:example@%s:%s/", host, port.Port())
	terminateFn := func() {
		_ = mongoC.Terminate(ctx)
	}

	return mongoURI, terminateFn, nil
}

// startServerWithEnv starts the application server with custom environment variables
func startServerWithEnv(ctx context.Context, t *testing.T, mongoURI string, extraEnv map[string]string) (string, *exec.Cmd, context.CancelFunc, *bytes.Buffer, error) {
	t.Helper()
	t.Log("Starting server")

	appPort, err := randomPort()
	if err != nil {
		return "", nil, nil, nil, err
	}

	// prepare /dev/null for stdout
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return "", nil, nil, nil, err
	}
	t.Cleanup(func() { _ = devNull.Close() })

	// Capture stderr for debugging with size limit
	const maxStderrSize = 64 * 1024 // 64KB max
	stderrBuf := &bytes.Buffer{}
	limitedStderr := &limitedWriter{w: stderrBuf, limit: maxStderrSize}

	// Add a small delay to ensure MongoDB is fully ready
	time.Sleep(2 * time.Second)

	srvCtx, srvCancel := context.WithCancel(ctx)

	bin := os.Getenv("BIN_SERVER")
	var cmd *exec.Cmd
	if bin != "" {
		// already compiled once in the workflow
		cmd = exec.CommandContext(srvCtx, bin)
	} else {
		// local `go test` fallback
		cmd = exec.CommandContext(srvCtx, "go", "run", "./cmd/server")
		cmd.Dir = "../"
	}

	// Make the wrapper the leader of a new process group
	// so that we can later send a signal to the whole tree.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	envVars := []string{
		fmt.Sprintf("MONGO_URI=%s", mongoURI),
		"MONGO_DB_NAME=e2e",
		"JWT_SECRET=test-e2e-secret-with-32-plus-characters-for-hs256-validation",
		"LOG_LEVEL=info", // reduce log noise
		fmt.Sprintf("APP_PORT=%s", appPort),
	}

	// Add extra environment variables
	for key, value := range extraEnv {
		envVars = append(envVars, fmt.Sprintf("%s=%s", key, value))
	}

	cmd.Env = append(envVars, os.Environ()...)
	cmd.Stdout = devNull // no extra goroutines, no console spam
	cmd.Stderr = limitedStderr

	t.Logf("Launching server on :%s (binary=%q)", appPort, bin)
	if err := cmd.Start(); err != nil {
		srvCancel()
		return "", nil, nil, nil, err
	}

	baseURL := fmt.Sprintf("http://localhost:%s", appPort)
	return baseURL, cmd, srvCancel, stderrBuf, nil
}

// waitHealthy waits for the server to respond to health checks
func waitHealthy(baseURL string, timeout time.Duration) error {
	healthURL := fmt.Sprintf("%s/healthz", baseURL)
	client := &http.Client{Timeout: 2 * time.Second}

	deadline := time.Now().UTC().Add(timeout)
	for {
		if time.Now().UTC().After(deadline) {
			return fmt.Errorf("server never responded on %s", healthURL)
		}

		resp, err := client.Get(healthURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// httpJSON performs an HTTP request with JSON payload and returns the response
func httpJSON(method, url string, payload any, headers map[string]string) (*http.Response, error) {
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, url, &body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	return client.Do(req)
}

// SetupTestEnvironment sets up the complete test environment
func SetupTestEnvironment(t *testing.T) *TestEnvironment {
	return SetupTestEnvironmentWithEnv(t, nil)
}

// SetupTestEnvironmentWithEnv sets up the complete test environment with custom environment variables
func SetupTestEnvironmentWithEnv(t *testing.T, extraEnv map[string]string) *TestEnvironment {
	t.Helper()
	t.Log("Setting up test environment")
	config.ResetCache()
	for k, v := range extraEnv {
		t.Setenv(k, v)
	}

	t.Cleanup(func() {
		config.ResetCache() // paranoia: wipe again when test ends
	})

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)

	mongoURI, mongoTerminate, err := startMongoTC(ctx, t)
	require.NoError(t, err)
	t.Cleanup(mongoTerminate)

	baseURL, cmd, srvCancel, stderrBuf, err := startServerWithEnv(ctx, t, mongoURI, extraEnv)
	require.NoError(t, err)

	// Set environment variables for the test process to access MongoDB
	t.Setenv("MONGO_URI", mongoURI)
	t.Setenv("MONGO_DB_NAME", "e2e")

	t.Cleanup(func() {
		srvCancel() // cancels the context (still keep it)

		// Best-effort: kill the entire process group (-pgid)
		if pgid, err := syscall.Getpgid(cmd.Process.Pid); err == nil {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		}

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

		// Dump stderr on cleanup if there's content
		if stderrBuf.Len() > 0 {
			t.Logf("Server stderr output (%d bytes):\n%s", stderrBuf.Len(), stderrBuf.String())
		}
	})

	if err := waitHealthy(baseURL, 30*time.Second); err != nil {
		// Dump stderr on health check failure
		if stderrBuf != nil && stderrBuf.Len() > 0 {
			t.Logf("Server stderr output on health check failure (%d bytes):\n%s", stderrBuf.Len(), stderrBuf.String())
		}
		require.NoError(t, err, "server never responded on %s", baseURL)
	}

	return &TestEnvironment{
		BaseURL: baseURL,
		Client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func signUp(t *testing.T, c *http.Client, baseURL, email, password string) {
	t.Helper()
	status, err := doJSONPost(t, c, baseURL+signUpEndpoint, map[string]string{
		"email":    email,
		"password": password,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)
}

func signInExpect(t *testing.T, c *http.Client, baseURL, email, password string, want int) {
	t.Helper()
	status, err := doJSONPost(t, c, baseURL+signInEndpoint, map[string]string{
		"email":    email,
		"password": password,
	})
	require.NoError(t, err)
	require.Equal(t, want, status)
}

func doJSONPost(t *testing.T, c *http.Client, url string, body any) (int, error) {
	t.Helper()
	b, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf(msgFailedToCloseResponseBody, err)
		}
	}()

	return resp.StatusCode, nil
}

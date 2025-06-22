package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/brianvoe/gofakeit/v6"
)

// ----------------------------------------------------------------------------
// Config ---------------------------------------------------------------------
var (
	baseURL = flag.String("url", env("API_BASE_URL", "http://localhost:8080"), "Server base URL")
	email   = flag.String("email", env("EMAIL", "demo@example.com"), "User e-mail")
	pass    = flag.String("pass", env("PASSWORD", "Password123"), "User password")
	nNotes  = flag.Int("n", envInt("COUNT", 500), "How many notes to create")

	pause  = flag.Duration("pause", envDuration("PAUSE", 0), "Base pause between requests, e.g. 200ms")
	jitter = flag.Float64("jitter", envFloat("JITTER", 0.3), "Jitter fraction applied to the pause, e.g. 0.3 for ±30 percent")
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= 0 {
			return d
		}
	}
	return def
}
func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			return f
		}
	}
	return def
}

func postJSON(path string, body any, hdr map[string]string) (*http.Response, error) {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, *baseURL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	return http.DefaultClient.Do(req)
}

func must(body io.ReadCloser) []byte {
	defer body.Close()
	data, _ := io.ReadAll(body)
	return data
}

func main() {
	flag.Parse()
	gofakeit.Seed(time.Now().UnixNano())

	fmt.Printf("Init account %s (notes=%d) on %s\n", *email, *nNotes, *baseURL)

	token, err := ensureUser()
	if err != nil {
		fmt.Fprintln(os.Stderr, "FATAL:", err)
		os.Exit(1)
	}

	if err := createNotes(token, *nNotes); err != nil {
		fmt.Fprintln(os.Stderr, "FATAL:", err)
		os.Exit(1)
	}

	fmt.Println("✔ done")
}

// Step 1: make sure the user exists
func ensureUser() (string, error) {
	payload := map[string]string{"email": *email, "password": *pass}

	// Try sign-up first …
	if resp, err := postJSON("/api/v1/auth/sign-up", payload, nil); err == nil && resp.StatusCode < 300 {
		var r struct {
			Token string `json:"token"`
		}
		_ = json.Unmarshal(must(resp.Body), &r)
		fmt.Println("• signed-up new user")
		return r.Token, nil
	}

	// … otherwise fall back to sign-in.
	resp, err := postJSON("/api/v1/auth/sign-in", payload, nil)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("sign-in failed (%d): %s", resp.StatusCode, must(resp.Body))
	}
	var r struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(must(resp.Body), &r)
	fmt.Println("• signed-in existing user")
	return r.Token, nil
}

// Step 2: create notes
func createNotes(token string, total int) error {
	h := map[string]string{"Authorization": "Bearer " + token}

	for i := 1; i <= total; i++ {
		note := map[string]any{
			"title": gofakeit.Sentence(3),
			"body":  gofakeit.Paragraph(1, 3, 40, " "),
			"color": gofakeit.HexColor(),
		}

		resp, err := postJSON("/api/v1/notes", note, h)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("create note %d failed (%d): %s", i, resp.StatusCode, must(resp.Body))
		}

		if i%50 == 0 || i == total {
			fmt.Printf("  … %d/%d\n", i, total)
		}

		// Optional jittered delay to ease load on the server
		if *pause > 0 {
			delta := float64(*pause) * *jitter
			sleep := *pause + time.Duration(rand.Float64()*2*delta-delta)
			if sleep > 0 {
				time.Sleep(sleep)
			}
		}
	}
	return nil
}

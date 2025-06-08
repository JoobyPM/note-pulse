package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
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
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var i int
		i, err := fmt.Sscan(v, &i)
		if err != nil {
			return def
		}
		if i > 0 {
			return i
		}
	}
	return def
}

// ----------------------------------------------------------------------------
// HTTP helpers ---------------------------------------------------------------
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

// ----------------------------------------------------------------------------
// Main -----------------------------------------------------------------------
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

// ----------------------------------------------------------------------------
// Step 1 – make sure the user exists -----------------------------------------
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

// ----------------------------------------------------------------------------
// Step 2 – create notes -------------------------------------------------------
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
	}
	return nil
}

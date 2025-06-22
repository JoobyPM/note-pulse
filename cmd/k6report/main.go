// cmd/k6report/main.go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type report struct {
	Metrics map[string]json.RawMessage `json:"metrics"`
}

type metric struct {
	P95        *float64                   `json:"p(95)"`
	P99        *float64                   `json:"p(99)"`
	Thresholds map[string]json.RawMessage `json:"thresholds"`
}

// helpers

func ms(f *float64) string {
	if f == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f", *f)
}

// true when **all** threshold expressions passed
func thresholdsOK(th map[string]json.RawMessage) bool {
	if len(th) == 0 {
		return false
	}

	for _, raw := range th {
		var old bool
		if err := json.Unmarshal(raw, &old); err == nil {
			if old { // true ⇒ failed
				return false
			}
			continue
		}

		// modern (≥ 0.45) - object with "ok"
		var obj struct {
			OK bool `json:"ok"`
		}
		if err := json.Unmarshal(raw, &obj); err != nil || !obj.OK {
			return false // err or ok:false ⇒ failed
		}
	}
	return true
}

// loadBaseline tries to read the previous report (committed version) to calculate deltas.
// It first checks K6REPORT_BASELINE env var; if absent, it picks the first *.report.md file found via git ls-files.
func loadBaseline() map[string]float64 {
	baseline := make(map[string]float64)

	path := os.Getenv("K6REPORT_BASELINE")
	if path == "" {
		out, err := exec.Command("git", "ls-files", "*.report.md").Output()
		if err != nil {
			return baseline
		}
		paths := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(paths) == 0 || paths[0] == "" {
			return baseline
		}
		path = paths[0]
	}

	content, err := exec.Command("git", "show", "HEAD:"+path).Output()
	if err != nil {
		// fallback to file on disk (useful when file is untracked)
		content, err = os.ReadFile(path)
		if err != nil {
			return baseline
		}
	}

	reRow := regexp.MustCompile(`^\|\s*([^|]+?)\s*\|\s*([0-9.]+|-)\s*\|`)
	for _, line := range strings.Split(string(content), "\n") {
		m := reRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		route := strings.TrimSpace(m[1])
		val := strings.TrimSpace(m[2])
		if val == "-" {
			continue
		}
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			baseline[route] = f
		}
	}
	return baseline
}

func main() {
	all, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}

	var rep report
	if err := json.Unmarshal(all, &rep); err != nil {
		panic(err)
	}

	reRoute := regexp.MustCompile(`^http_req_duration\{route:([^}]+)\}$`)

	type row struct {
		name, p95, delta, p99 string
		ok                    bool
	}
	var rows []row
	var failRateOK *bool

	// load old values once
	old := loadBaseline()

	for name, raw := range rep.Metrics {
		switch {
		case name == "http_req_failed":
			var m metric
			if json.Unmarshal(raw, &m) == nil {
				ok := thresholdsOK(m.Thresholds)
				failRateOK = &ok
			}

		case reRoute.MatchString(name):
			var m metric
			if json.Unmarshal(raw, &m) != nil {
				continue
			}
			route := reRoute.FindStringSubmatch(name)[1]

			// calculate delta vs baseline p95
			delta := "-"
			if base, ok := old[route]; ok && m.P95 != nil {
				diff := *m.P95 - base
				delta = fmt.Sprintf("%+.2f", diff)
			}

			rows = append(rows, row{
				name:  route,
				p95:   ms(m.P95),
				delta: delta,
				p99:   ms(m.P99),
				ok:    thresholdsOK(m.Thresholds),
			})
		}
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })

	fmt.Println("# k6 latency targets report")
	fmt.Println("")
	fmt.Println("| route | p95 (ms) | +/- ms | p99 (ms) | target |")
	fmt.Println("|-------|---------:|-------:|---------:|:---:|")
	for _, r := range rows {
		mark := "❌"
		if r.ok {
			mark = "✅"
		}
		fmt.Printf("| %s | %s | %s | %s | %s |\n", r.name, r.p95, r.delta, r.p99, mark)
	}

	fmt.Println("\n---")
	if failRateOK != nil {
		mark := "❌"
		if *failRateOK {
			mark = "✅"
		}
		fmt.Printf("**http_req_failed**: - (target %s)\n", mark)
	}
}

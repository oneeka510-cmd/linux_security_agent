package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultConfigPath = "/etc/go-agent/config.json"
	maxAttempts       = 3
	requestTimeout    = 60 * time.Second
)

// Config says where to send reports. The API key is a secret, so it never lives in the code.
type Config struct {
	APIURL string `json:"api_url"`
	APIKey string `json:"api_key"`
}

// loadConfig reads the config file, then lets environment variables override it.
//
//	AGENT_CONFIG    path of the config file (default /etc/go-agent/config.json)
//	AGENT_API_URL   overrides api_url
//	AGENT_API_KEY   overrides api_key
func loadConfig() (Config, error) {
	var cfg Config

	path := os.Getenv("AGENT_CONFIG")
	if path == "" {
		path = defaultConfigPath
	}
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("config file %s is not valid JSON: %w", path, err)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return cfg, fmt.Errorf("reading config file %s: %w", path, err)
	}

	if v := os.Getenv("AGENT_API_URL"); v != "" {
		cfg.APIURL = v
	}
	if v := os.Getenv("AGENT_API_KEY"); v != "" {
		cfg.APIKey = v
	}

	cfg.APIURL = strings.TrimRight(cfg.APIURL, "/")
	if cfg.APIURL == "" || cfg.APIKey == "" {
		return cfg, fmt.Errorf("api_url and api_key are required: set them in %s or in AGENT_API_URL / AGENT_API_KEY", path)
	}
	return cfg, nil
}

// sendReport uploads the report to POST <api_url>/ingest and returns the server's reply.
// Network problems and server errors (5xx) are retried; client errors (4xx, e.g. a wrong
// API key) are not, because trying again cannot fix them.
func sendReport(cfg Config, report Report) (string, error) {
	body, err := json.Marshal(report)
	if err != nil {
		return "", fmt.Errorf("encoding report: %w", err)
	}

	client := &http.Client{Timeout: requestTimeout}
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		req, err := http.NewRequest(http.MethodPost, cfg.APIURL+"/ingest", bytes.NewReader(body))
		if err != nil {
			return "", fmt.Errorf("building request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", cfg.APIKey)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
		} else {
			reply, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
			text := strings.TrimSpace(string(reply))

			switch {
			case resp.StatusCode >= 200 && resp.StatusCode < 300:
				return text, nil
			case resp.StatusCode >= 400 && resp.StatusCode < 500:
				return "", fmt.Errorf("server rejected the report (HTTP %d): %s", resp.StatusCode, text)
			default:
				lastErr = fmt.Errorf("server error (HTTP %d): %s", resp.StatusCode, text)
			}
		}

		if attempt < maxAttempts {
			fmt.Fprintf(os.Stderr, "attempt %d of %d failed: %v; retrying\n", attempt, maxAttempts, lastErr)
			time.Sleep(time.Duration(attempt*2) * time.Second)
		}
	}
	return "", fmt.Errorf("giving up after %d attempts: %w", maxAttempts, lastErr)
}

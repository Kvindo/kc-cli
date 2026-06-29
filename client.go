package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type cacheEntry struct {
	Body    []byte    `json:"body"`
	Expires time.Time `json:"expires"`
}

func post(server, token string, args []string, retry *RetryParams, editApply *EditApply, fileContent string) (int, []byte, error) {
	// Polls (retry), edit-applies and apply file-reads must always hit the server; only fresh
	// commands use the cache.
	if retry == nil && editApply == nil && fileContent == "" {
		if cached := readCache(args); cached != nil {
			return 200, cached, nil
		}
	}

	reqBody := map[string]any{"args": args, "retryParams": retry, "editApply": editApply}
	if fileContent != "" {
		reqBody["fileContent"] = fileContent
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return 0, nil, fmt.Errorf("marshal args: %w", err)
	}

	req, err := http.NewRequest("POST", server+"/api/v1/cli", bytes.NewReader(payload))
	if err != nil {
		return 0, nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response: %w", err)
	}
	body := buf.Bytes()

	if resp.StatusCode == 200 && retry == nil && editApply == nil && fileContent == "" {
		writeCache(args, body, resp.Header.Get("Cache-Control"))
	}

	return resp.StatusCode, body, nil
}

// postRetrying wraps post() with up to postMaxAttempts attempts on transient failures:
// network errors, HTTP 5xx, empty body, or a non-JSON 200 (e.g. an nginx/envoy error page). 4xx
// responses are intentional (401 triggers a token prompt; 400/404 are user errors) and returned
// immediately without retry. Mirrors the retry logic already in the E2E test helper (PostCliAsync).
const postMaxAttempts = 4
const postRetryDelay = 2 * time.Second

func postRetrying(server, token string, args []string, retry *RetryParams, editApply *EditApply, fileContent string) (int, []byte, error) {
	var (
		status int
		body   []byte
		err    error
	)
	for attempt := 0; attempt < postMaxAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(postRetryDelay)
		}
		status, body, err = post(server, token, args, retry, editApply, fileContent)
		if !isTransientFailure(status, body, err) {
			return status, body, err
		}
	}
	return status, body, err
}

func isTransientFailure(status int, body []byte, err error) bool {
	if err != nil {
		return true
	}
	if status >= 500 {
		return true
	}
	if len(body) == 0 {
		return true
	}
	// 200 with a non-JSON body = nginx/envoy served an HTML error page instead of the API response
	if status == 200 && !json.Valid(body) {
		return true
	}
	return false
}

func cacheKey(args []string) string {
	data, _ := json.Marshal(args)
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum)
}

func readCache(args []string) []byte {
	path := filepath.Join(cacheDir(), cacheKey(args)+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil
	}
	if time.Now().After(entry.Expires) {
		_ = os.Remove(path)
		return nil
	}
	return entry.Body
}

func writeCache(args []string, body []byte, cacheControl string) {
	maxAge := parseCacheControlMaxAge(cacheControl)
	if maxAge <= 0 {
		return
	}
	if err := os.MkdirAll(cacheDir(), 0700); err != nil {
		return
	}
	entry := cacheEntry{
		Body:    body,
		Expires: time.Now().Add(time.Duration(maxAge) * time.Second),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(cacheDir(), cacheKey(args)+".json"), data, 0600)
}

func parseCacheControlMaxAge(header string) int {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "max-age=") {
			v, err := strconv.Atoi(strings.TrimPrefix(part, "max-age="))
			if err == nil {
				return v
			}
		}
	}
	return 0
}

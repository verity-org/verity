package preflight

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ManifestEntry stores the state of a single image from the last successful
// patch/build cycle.
type ManifestEntry struct {
	UpstreamDigest string `json:"upstream_digest"`
	PatchedVulns   int    `json:"patched_vulns"`
	LastPatched    string `json:"last_patched,omitempty"`
}

// Manifest maps "image/tag" keys to their last-known state.
type Manifest map[string]ManifestEntry

// ManifestKey returns the canonical manifest key for an image and tag.
func ManifestKey(image, tag string) string {
	return image + "/" + tag
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

// fetchManifestURL builds the GitHub Contents API URL. It is a package-level
// variable so tests can redirect requests to an httptest server.
var fetchManifestURL = func(repo, branch string) string {
	return fmt.Sprintf(
		"https://api.github.com/repos/%s/contents/reports/preflight-manifest.json?ref=%s",
		repo, branch,
	)
}

// FetchManifest retrieves the preflight manifest from the GitHub Contents API.
// If the file does not exist (404), an empty manifest is returned.
func FetchManifest(repo, branch, token string) (Manifest, error) {
	url := fetchManifestURL(repo, branch)

	req, err := http.NewRequestWithContext(context.Background(), "GET", url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating manifest request: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "token "+token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := httpClient.Do(req) //nolint:gosec // URL built from repo/branch inputs
	if err != nil {
		return nil, fmt.Errorf("fetching manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return Manifest{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching manifest: %w: %d", ErrUnexpectedStatus, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading manifest response: %w", err)
	}

	// GitHub Contents API returns { "content": "<base64>", "encoding": "base64" }
	var ghResp struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(body, &ghResp); err != nil {
		return nil, fmt.Errorf("parsing GitHub response: %w", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(ghResp.Content)
	if err != nil {
		return nil, fmt.Errorf("decoding manifest content: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(decoded, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest JSON: %w", err)
	}

	return m, nil
}

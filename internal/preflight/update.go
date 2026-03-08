package preflight

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrUnexpectedStatus is returned when the GitHub API responds with a
// non-success HTTP status code.
var ErrUnexpectedStatus = errors.New("unexpected HTTP status")

var fetchManifestWithSHAURL = func(repo, branch string) string {
	return fmt.Sprintf(
		"https://api.github.com/repos/%s/contents/reports/preflight-manifest.json?ref=%s",
		repo, branch,
	)
}

var pushManifestURL = func(repo string) string {
	return fmt.Sprintf(
		"https://api.github.com/repos/%s/contents/reports/preflight-manifest.json",
		repo,
	)
}

// UpdateEntry is the payload for updating a single entry in the manifest.
type UpdateEntry struct {
	ImageName      string
	Tag            string
	UpstreamDigest string
	PatchedVulns   int
}

// UpdateManifest merges the given entry into the remote preflight manifest
// on the reports branch via the GitHub Contents API.
func UpdateManifest(repo, branch, token string, entry UpdateEntry) error {
	key := ManifestKey(entry.ImageName, entry.Tag)

	manifest, sha, err := fetchManifestWithSHA(repo, branch, token)
	if err != nil {
		return fmt.Errorf("fetching current manifest: %w", err)
	}

	manifest[key] = ManifestEntry{
		UpstreamDigest: entry.UpstreamDigest,
		PatchedVulns:   entry.PatchedVulns,
		LastPatched:    time.Now().UTC().Format(time.RFC3339),
	}

	return pushManifest(repo, branch, token, sha, manifest, key)
}

// fetchManifestWithSHA retrieves the manifest and its git SHA for conditional updates.
func fetchManifestWithSHA(repo, branch, token string) (Manifest, string, error) {
	url := fetchManifestWithSHAURL(repo, branch)

	req, err := http.NewRequestWithContext(context.Background(), "GET", url, http.NoBody)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := httpClient.Do(req) //nolint:gosec // URL built from repo/branch inputs
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return Manifest{}, "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("%w: %d", ErrUnexpectedStatus, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	var ghResp struct {
		Content string `json:"content"`
		SHA     string `json:"sha"`
	}
	if err := json.Unmarshal(body, &ghResp); err != nil {
		return nil, "", err
	}

	decoded, err := base64.StdEncoding.DecodeString(ghResp.Content)
	if err != nil {
		return nil, "", err
	}

	var m Manifest
	if err := json.Unmarshal(decoded, &m); err != nil {
		return nil, "", err
	}

	return m, ghResp.SHA, nil
}

// pushManifest writes the updated manifest back via the GitHub Contents API.
func pushManifest(repo, branch, token, sha string, manifest Manifest, key string) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling manifest: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(data)

	payload := map[string]string{
		"message": "chore: update preflight manifest for " + key,
		"content": encoded,
		"branch":  branch,
	}
	if sha != "" {
		payload["sha"] = sha
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := pushManifestURL(repo)

	req, err := http.NewRequestWithContext(context.Background(), "PUT", url, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req) //nolint:gosec // URL built from repo input
	if err != nil {
		return fmt.Errorf("pushing manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("pushing manifest: %w: %d (body unreadable)", ErrUnexpectedStatus, resp.StatusCode)
		}
		return fmt.Errorf("pushing manifest: %w: %d: %s", ErrUnexpectedStatus, resp.StatusCode, string(body))
	}

	return nil
}

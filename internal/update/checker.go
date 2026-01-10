// Package update provides version checking against GitHub releases.
// checker.go: The "Update Sentinel"
//
// Term-Phase 11: Production Release Pipeline
// Checks GitHub API for new releases and notifies via Wails events.
//
// Design Principles:
//   - Fail silently (no crashes if offline)
//   - Lightweight goroutine (non-blocking)
//   - Semantic version comparison
//   - Wails event integration

package update

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ═══════════════════════════════════════════════════════════════
// CONSTANTS
// ═══════════════════════════════════════════════════════════════

const (
	// CurrentVersion is the embedded version string.
	// Updated at build time or manually before release.
	CurrentVersion = "1.0.0"

	// GitHubRepo is the repository to check for updates.
	GitHubRepo = "HarshalPatel1972/aero"

	// CheckInterval is how often to check for updates.
	CheckInterval = 24 * time.Hour

	// Timeout for GitHub API requests.
	RequestTimeout = 10 * time.Second
)

// ═══════════════════════════════════════════════════════════════
// TYPES
// ═══════════════════════════════════════════════════════════════

// GitHubRelease represents the API response.
type GitHubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Prerelease  bool   `json:"prerelease"`
	Draft       bool   `json:"draft"`
}

// UpdateInfo contains information about an available update.
type UpdateInfo struct {
	Available       bool   `json:"available"`
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	ReleaseURL      string `json:"releaseUrl"`
	ReleaseNotes    string `json:"releaseNotes"`
}

// Checker handles update checking.
type Checker struct {
	ctx        context.Context
	cancelFunc context.CancelFunc
	appCtx     context.Context // Wails app context
	repo       string
	current    string
	lastCheck  time.Time
	lastResult *UpdateInfo
}

// ═══════════════════════════════════════════════════════════════
// CHECKER
// ═══════════════════════════════════════════════════════════════

// NewChecker creates a new update checker.
func NewChecker() *Checker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Checker{
		ctx:        ctx,
		cancelFunc: cancel,
		repo:       GitHubRepo,
		current:    CurrentVersion,
	}
}

// SetAppContext sets the Wails app context for event emission.
func (c *Checker) SetAppContext(ctx context.Context) {
	c.appCtx = ctx
}

// Start begins the background update checking routine.
func (c *Checker) Start() {
	go c.backgroundCheck()
	log.Printf("[UPDATE] 🔍 Update sentinel started (v%s)", c.current)
}

// Stop halts the background checker.
func (c *Checker) Stop() {
	c.cancelFunc()
	log.Printf("[UPDATE] 🛑 Update sentinel stopped")
}

// Check performs an immediate update check.
func (c *Checker) Check() (*UpdateInfo, error) {
	return c.checkNow()
}

// GetCurrentVersion returns the current version string.
func (c *Checker) GetCurrentVersion() string {
	return c.current
}

// ═══════════════════════════════════════════════════════════════
// INTERNAL
// ═══════════════════════════════════════════════════════════════

// backgroundCheck runs periodic update checks.
func (c *Checker) backgroundCheck() {
	// Initial check after 5 seconds (let app settle)
	time.Sleep(5 * time.Second)
	
	if info, err := c.checkNow(); err == nil && info.Available {
		c.emitUpdateEvent(info)
	}

	// Periodic checks
	ticker := time.NewTicker(CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if info, err := c.checkNow(); err == nil && info.Available {
				c.emitUpdateEvent(info)
			}
		}
	}
}

// checkNow performs the actual GitHub API request.
func (c *Checker) checkNow() (*UpdateInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", c.repo)

	ctx, cancel := context.WithTimeout(c.ctx, RequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Printf("[UPDATE] ⚠️ Failed to create request: %v", err)
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Aero-UpdateChecker/"+c.current)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Fail silently - no internet is not an error
		log.Printf("[UPDATE] ⚠️ Network unavailable, skipping check")
		return &UpdateInfo{Available: false, CurrentVersion: c.current}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[UPDATE] ⚠️ GitHub API returned %d", resp.StatusCode)
		return &UpdateInfo{Available: false, CurrentVersion: c.current}, nil
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		log.Printf("[UPDATE] ⚠️ Failed to parse response: %v", err)
		return nil, err
	}

	// Skip prereleases and drafts
	if release.Prerelease || release.Draft {
		return &UpdateInfo{Available: false, CurrentVersion: c.current}, nil
	}

	// Parse version from tag (strip 'v' prefix if present)
	latestVersion := strings.TrimPrefix(release.TagName, "v")

	// Compare versions
	isNewer := compareVersions(latestVersion, c.current) > 0

	info := &UpdateInfo{
		Available:      isNewer,
		CurrentVersion: c.current,
		LatestVersion:  latestVersion,
		ReleaseURL:     release.HTMLURL,
		ReleaseNotes:   release.Body,
	}

	c.lastCheck = time.Now()
	c.lastResult = info

	if isNewer {
		log.Printf("[UPDATE] 🆕 New version available: %s (current: %s)", latestVersion, c.current)
	} else {
		log.Printf("[UPDATE] ✅ Up to date (v%s)", c.current)
	}

	return info, nil
}

// emitUpdateEvent sends a Wails event to the frontend.
func (c *Checker) emitUpdateEvent(info *UpdateInfo) {
	if c.appCtx == nil {
		return
	}
	runtime.EventsEmit(c.appCtx, "system:update_available", info)
}

// ═══════════════════════════════════════════════════════════════
// VERSION COMPARISON
// ═══════════════════════════════════════════════════════════════

// compareVersions compares two semantic version strings.
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func compareVersions(a, b string) int {
	aParts := parseVersion(a)
	bParts := parseVersion(b)

	for i := 0; i < 3; i++ {
		if aParts[i] > bParts[i] {
			return 1
		}
		if aParts[i] < bParts[i] {
			return -1
		}
	}
	return 0
}

// parseVersion extracts major, minor, patch from a version string.
func parseVersion(v string) [3]int {
	parts := strings.Split(v, ".")
	var result [3]int

	for i := 0; i < len(parts) && i < 3; i++ {
		// Strip any suffix (e.g., "0-beta" -> "0")
		numStr := strings.Split(parts[i], "-")[0]
		if n, err := strconv.Atoi(numStr); err == nil {
			result[i] = n
		}
	}

	return result
}

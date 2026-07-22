package version

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	githubAPI  = "https://api.github.com/repos/aurorallm/aurora/releases/latest"
	checkDelay = 800 * time.Millisecond
)

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

func CheckForUpdate(ctx context.Context) (string, bool) {
	if Version == "dev" || strings.HasPrefix(Version, "0.") {
		return "", false
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPI, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "aurora-cli")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", false
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(Version, "v")

	if compareSemver(current, latest) < 0 {
		msg := fmt.Sprintf(
			"\n  \033[38;5;220m⚠ Update available:\033[0m \033[38;5;106m%s\033[0m → \033[38;5;39m%s\033[0m\n  \033[38;5;243mRun \033[0m\033[38;5;39maurora update\033[0m\033[38;5;243m to upgrade, or see:\033[0m\n  \033[38;5;39m%s\033[0m\n",
			current, latest, release.HTMLURL,
		)
		return msg, true
	}

	return "", false
}

func CheckForUpdatePlain() (string, bool) {
	if Version == "dev" || strings.HasPrefix(Version, "0.") {
		return "", false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPI, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "aurora-cli")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", false
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(Version, "v")

	if compareSemver(current, latest) < 0 {
		return fmt.Sprintf("Update available: %s → %s (run `aurora update` or see %s)",
			current, latest, release.HTMLURL), true
	}

	return "", false
}

func compareSemver(a, b string) int {
	an := parseVersion(a)
	bn := parseVersion(b)
	for i := 0; i < 3; i++ {
		if an[i] < bn[i] {
			return -1
		}
		if an[i] > bn[i] {
			return 1
		}
	}
	return 0
}

func parseVersion(s string) [3]int {
	parts := strings.SplitN(s, ".", 3)
	var v [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		n, err := strconv.Atoi(strings.TrimSpace(parts[i]))
		if err == nil {
			v[i] = n
		}
	}
	return v
}

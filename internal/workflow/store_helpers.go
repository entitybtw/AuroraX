package workflow

import (
	"fmt"
	"strings"
)

type rowScanner interface {
	Scan(dest ...any) error
}

type rowIterator interface {
	rowScanner
	Next() bool
	Err() error
}

func fetchVersions(rows rowIterator, scan func(rowScanner) (Version, error)) ([]Version, error) {
	versions := make([]Version, 0)
	for rows.Next() {
		version, err := scan(rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workflows: %w", err)
	}
	return versions, nil
}

func buildStoredPath(scopeKey, userPath string) string {
	userPath = strings.TrimSpace(userPath)
	if userPath != "" {
		return userPath
	}

	switch {
	case strings.HasPrefix(scopeKey, "path:"):
		path := strings.TrimPrefix(scopeKey, "path:")
		if path == "" {
			return "/"
		}
		return path
	case strings.HasPrefix(scopeKey, "provider_path:"):
		parts := strings.SplitN(scopeKey, ":", 3)
		if len(parts) == 3 {
			if strings.TrimSpace(parts[2]) == "" {
				return "/"
			}
			return parts[2]
		}
	case strings.HasPrefix(scopeKey, "provider_model_path:"):
		parts := strings.SplitN(scopeKey, ":", 4)
		if len(parts) == 4 {
			if strings.TrimSpace(parts[3]) == "" {
				return "/"
			}
			return parts[3]
		}
	}

	return ""
}

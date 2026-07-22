package version

import (
	"fmt"
	"runtime"
	"strings"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

type BuildInfo struct {
	Version   string
	Commit    string
	Date      string
	GoVersion string
	Platform  string
}

func Current() BuildInfo {
	return BuildInfo{
		Version:   Version,
		Commit:    Commit,
		Date:      Date,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

func Info() string {
	var parts []string
	if Version != "dev" {
		parts = append(parts, Version)
	}
	if Commit != "none" {
		parts = append(parts, fmt.Sprintf("commit:%s", trimShort(Commit)))
	}
	parts = append(parts, runtime.Version())
	return fmt.Sprintf("aurora [%s]", strings.Join(parts, " | "))
}

func trimShort(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

package updater

import (
	"strconv"
	"strings"
)

// CurrentVersion is the active version of LocalArchive.
// This can be set at compile time via -ldflags "-X local-archive/internal/updater.CurrentVersion=x.y.z".
var CurrentVersion = "0.1.8"

// CompareVersions compares two semantic version strings (e.g. "0.1.2", "v0.1.3", "1.0.0-rc1").
// Returns:
//
//	 1 if v1 > v2
//	-1 if v1 < v2
//	 0 if v1 == v2
func CompareVersions(v1, v2 string) int {
	clean1 := cleanVersionString(v1)
	clean2 := cleanVersionString(v2)

	core1, pre1 := splitPrerelease(clean1)
	core2, pre2 := splitPrerelease(clean2)

	parts1 := strings.Split(core1, ".")
	parts2 := strings.Split(core2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			n1, _ = strconv.Atoi(strings.TrimSpace(parts1[i]))
		}
		if i < len(parts2) {
			n2, _ = strconv.Atoi(strings.TrimSpace(parts2[i]))
		}

		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}

	// If core versions are identical, a version with a prerelease is lower than one without
	if pre1 != "" && pre2 == "" {
		return -1
	}
	if pre1 == "" && pre2 != "" {
		return 1
	}
	if pre1 != "" && pre2 != "" {
		if pre1 > pre2 {
			return 1
		}
		if pre1 < pre2 {
			return -1
		}
	}

	return 0
}

func cleanVersionString(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return strings.TrimSpace(v)
}

func splitPrerelease(v string) (core, prerelease string) {
	// Remove build metadata (+...)
	if idx := strings.Index(v, "+"); idx != -1 {
		v = v[:idx]
	}
	// Split prerelease (-...)
	if idx := strings.Index(v, "-"); idx != -1 {
		return v[:idx], v[idx+1:]
	}
	return v, ""
}

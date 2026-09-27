package updater

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"0.1.2", "0.1.2", 0},
		{"v0.1.2", "0.1.2", 0},
		{"V0.1.2", "v0.1.2", 0},
		{"0.1.3", "0.1.2", 1},
		{"0.1.2", "0.1.3", -1},
		{"0.2.0", "0.1.9", 1},
		{"1.0.0", "0.9.9", 1},
		{"0.1.10", "0.1.2", 1},
		{"0.1.2", "0.1.10", -1},
		{"1.0.0", "1.0", 0},
		{"1.0.0", "1.0.0.0", 0},
		{"0.1.2", "0.1.2-beta", 1},
		{"0.1.2-alpha", "0.1.2", -1},
		{"0.1.2-alpha", "0.1.2-beta", -1},
		{"0.1.3-beta", "0.1.2", 1},
	}

	for _, tt := range tests {
		got := CompareVersions(tt.v1, tt.v2)
		if got != tt.expected {
			t.Errorf("CompareVersions(%q, %q) = %d, expected %d", tt.v1, tt.v2, got, tt.expected)
		}
	}
}

func TestFindInstallerAsset(t *testing.T) {
	t.Run("picks installer named LocalArchive-amd64-installer.exe", func(t *testing.T) {
		assets := []GitHubAsset{
			{Name: "LocalArchive.exe", BrowserDownloadURL: "http://example.com/bin.exe"},
			{Name: "LocalArchive-amd64-installer.exe", BrowserDownloadURL: "http://example.com/installer.exe"},
		}
		found, err := FindInstallerAsset(assets)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found.Name != "LocalArchive-amd64-installer.exe" {
			t.Fatalf("expected LocalArchive-amd64-installer.exe, got %s", found.Name)
		}
	})

	t.Run("picks setup named LocalArchive-Setup.exe", func(t *testing.T) {
		assets := []GitHubAsset{
			{Name: "LocalArchive-Setup.exe", BrowserDownloadURL: "http://example.com/setup.exe"},
			{Name: "LocalArchive.exe", BrowserDownloadURL: "http://example.com/bin.exe"},
		}
		found, err := FindInstallerAsset(assets)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if found.Name != "LocalArchive-Setup.exe" {
			t.Fatalf("expected LocalArchive-Setup.exe, got %s", found.Name)
		}
	})

	t.Run("errors when no exe asset", func(t *testing.T) {
		assets := []GitHubAsset{
			{Name: "source.zip", BrowserDownloadURL: "http://example.com/source.zip"},
		}
		_, err := FindInstallerAsset(assets)
		if err == nil {
			t.Fatalf("expected error when no .exe is found")
		}
	})
}

func TestCheckForUpdatesWithMockServer(t *testing.T) {
	mockRelease := GitHubRelease{
		TagName:     "0.1.3",
		Name:        "Local Archive 0.1.3",
		Body:        "Fixes and improvements",
		PublishedAt: "2026-09-27T10:00:00Z",
		Assets: []GitHubAsset{
			{
				Name:               "LocalArchive-amd64-installer.exe",
				Size:               8512735,
				BrowserDownloadURL: "http://example.com/LocalArchive-amd64-installer.exe",
			},
			{
				Name:               "LocalArchive.exe",
				Size:               16188928,
				BrowserDownloadURL: "http://example.com/LocalArchive.exe",
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(mockRelease)
	}))
	defer server.Close()

	// Replace HTTP transport to route api.github.com to mock server
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			newReq, _ := http.NewRequestWithContext(req.Context(), req.Method, server.URL, req.Body)
			return http.DefaultTransport.RoundTrip(newReq)
		}),
	}

	info, err := CheckForUpdates(client, "0.1.2", "sadiqAlAboudi", "local-archive")
	if err != nil {
		t.Fatalf("CheckForUpdates failed: %v", err)
	}

	if !info.Available {
		t.Fatalf("expected update to be available for current=0.1.2 and latest=0.1.3")
	}
	if info.LatestVersion != "0.1.3" {
		t.Fatalf("expected latest version 0.1.3, got %s", info.LatestVersion)
	}
	if info.DownloadURL != "http://example.com/LocalArchive-amd64-installer.exe" {
		t.Fatalf("unexpected download url: %s", info.DownloadURL)
	}

	// Now check when already up to date
	infoSame, err := CheckForUpdates(client, "0.1.3", "sadiqAlAboudi", "local-archive")
	if err != nil {
		t.Fatalf("CheckForUpdates failed: %v", err)
	}
	if infoSame.Available {
		t.Fatalf("expected update NOT to be available when versions match")
	}
}

func TestDownloadInstaller(t *testing.T) {
	fileContent := []byte("MZ...This is a simulated LocalArchive installer executable content")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "66")
		_, _ = w.Write(fileContent)
	}))
	defer server.Close()

	var progressReports int
	var lastPercent float64

	dest, err := DownloadInstaller(context.Background(), server.Client(), server.URL, func(downloaded, total int64, percent float64) {
		progressReports++
		lastPercent = percent
	})
	if err != nil {
		t.Fatalf("DownloadInstaller failed: %v", err)
	}
	defer os.Remove(dest)

	expectedPath := filepath.Join(os.TempDir(), TargetInstallerFileName)
	if dest != expectedPath {
		t.Fatalf("expected dest %q, got %q", expectedPath, dest)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("failed to read downloaded installer: %v", err)
	}
	if string(data) != string(fileContent) {
		t.Fatalf("downloaded content mismatch")
	}
	if progressReports == 0 {
		t.Fatalf("expected at least one progress callback")
	}
	if lastPercent != 100.0 {
		t.Fatalf("expected final percent to be 100.0, got %f", lastPercent)
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

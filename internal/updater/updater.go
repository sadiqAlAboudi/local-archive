package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"local-archive/internal/sysutil"
)

const (
	// DefaultRepoOwner is the GitHub repository owner.
	DefaultRepoOwner = "sadiqAlAboudi"
	// DefaultRepoName is the GitHub repository name.
	DefaultRepoName = "local-archive"
	// TargetInstallerFileName is the required installer filename in %TEMP%.
	TargetInstallerFileName = "LocalArchive-Setup.exe"
)

// GitHubAsset represents a downloadable release artifact on GitHub.
type GitHubAsset struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
	ContentType        string `json:"content_type"`
}

// GitHubRelease represents a GitHub release payload.
type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	Draft       bool          `json:"draft"`
	Prerelease  bool          `json:"prerelease"`
	PublishedAt string        `json:"published_at"`
	Assets      []GitHubAsset `json:"assets"`
}

// UpdateInfo contains update availability and metadata for the frontend.
type UpdateInfo struct {
	Available      bool   `json:"available"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	ReleaseTitle   string `json:"release_title"`
	PublishedAt    string `json:"published_at"`
	DownloadURL    string `json:"download_url"`
	AssetSize      int64  `json:"asset_size"`
	FormattedSize  string `json:"formatted_size"`
	AssetName      string `json:"asset_name"`
}

// FindInstallerAsset identifies the Windows NSIS installer among the release assets.
func FindInstallerAsset(assets []GitHubAsset) (*GitHubAsset, error) {
	if len(assets) == 0 {
		return nil, fmt.Errorf("لا توجد ملفات مرفقة في هذا الإصدار")
	}

	// 1. Look for asset ending with .exe and containing 'installer' or 'setup'
	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		if strings.HasSuffix(name, ".exe") && (strings.Contains(name, "installer") || strings.Contains(name, "setup")) {
			return &assets[i], nil
		}
	}

	// 2. Look for any .exe that is NOT the raw portable executable "LocalArchive.exe"
	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		if strings.HasSuffix(name, ".exe") && name != "localarchive.exe" {
			return &assets[i], nil
		}
	}

	// 3. Fallback: first .exe file found
	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		if strings.HasSuffix(name, ".exe") {
			return &assets[i], nil
		}
	}

	return nil, fmt.Errorf("لم يتم العثور على مثبت البرنامج (.exe) في أصول هذا الإصدار")
}

// CheckForUpdates queries the GitHub API for the latest release and compares versions.
func CheckForUpdates(client *http.Client, currentVersion, repoOwner, repoName string) (*UpdateInfo, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	if repoOwner == "" {
		repoOwner = DefaultRepoOwner
	}
	if repoName == "" {
		repoName = DefaultRepoName
	}
	if currentVersion == "" {
		currentVersion = CurrentVersion
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("فشل تجهيز طلب التحديث: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", fmt.Sprintf("LocalArchive-Updater/%s", currentVersion))

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("تعذر الاتصال بـ GitHub للتحقق من التحديثات: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("لم يتم العثور على إصدارات منشورة للمستودع")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("استجاب خادم GitHub برمز الخطأ: %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("فشل تحليل بيانات الإصدار من GitHub: %w", err)
	}

	installerAsset, err := FindInstallerAsset(release.Assets)
	if err != nil {
		return nil, err
	}

	latestTag := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(release.TagName), "v"), "V")
	isNewer := CompareVersions(release.TagName, currentVersion) > 0

	releaseTitle := release.Name
	if releaseTitle == "" {
		releaseTitle = release.TagName
	}

	return &UpdateInfo{
		Available:      isNewer,
		CurrentVersion: currentVersion,
		LatestVersion:  latestTag,
		ReleaseTitle:   releaseTitle,
		PublishedAt:    release.PublishedAt,
		DownloadURL:    installerAsset.BrowserDownloadURL,
		AssetSize:      installerAsset.Size,
		FormattedSize:  sysutil.FormatBytes(installerAsset.Size),
		AssetName:      installerAsset.Name,
	}, nil
}

// DownloadInstaller downloads the installer file to %TEMP%\LocalArchive-Setup.exe with progress.
func DownloadInstaller(ctx context.Context, client *http.Client, downloadURL string, progressFn func(downloaded, total int64, percent float64)) (string, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Minute}
	}

	tempDir := os.TempDir()
	targetPath := filepath.Join(tempDir, TargetInstallerFileName)
	partPath := targetPath + ".download"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("تعذر تجهيز طلب تنزيل التحديث: %w", err)
	}
	req.Header.Set("User-Agent", "LocalArchive-Updater")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("فشل تنزيل ملف التحديث: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("فشل تنزيل التحديث برمز استجابة غير متوقع: %d", resp.StatusCode)
	}

	totalSize := resp.ContentLength

	outFile, err := os.Create(partPath)
	if err != nil {
		return "", fmt.Errorf("تعذر إنشاء ملف التحديث المؤقت: %w", err)
	}

	var downloaded int64
	buf := make([]byte, 32*1024)
	lastReport := time.Now()

	for {
		select {
		case <-ctx.Done():
			outFile.Close()
			_ = os.Remove(partPath)
			return "", ctx.Err()
		default:
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := outFile.Write(buf[:n]); writeErr != nil {
				outFile.Close()
				_ = os.Remove(partPath)
				return "", fmt.Errorf("خطأ أثناء كتابة ملف التحديث: %w", writeErr)
			}
			downloaded += int64(n)

			if progressFn != nil && (time.Since(lastReport) > 100*time.Millisecond || readErr == io.EOF) {
				var percent float64
				if totalSize > 0 {
					percent = float64(downloaded) / float64(totalSize) * 100.0
					if percent > 100.0 {
						percent = 100.0
					}
				}
				progressFn(downloaded, totalSize, percent)
				lastReport = time.Now()
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			outFile.Close()
			_ = os.Remove(partPath)
			return "", fmt.Errorf("انقطع تنزيل ملف التحديث: %w", readErr)
		}
	}

	if err := outFile.Sync(); err != nil {
		outFile.Close()
		_ = os.Remove(partPath)
		return "", fmt.Errorf("تعذر حفظ بيانات ملف التحديث: %w", err)
	}
	if err := outFile.Close(); err != nil {
		_ = os.Remove(partPath)
		return "", fmt.Errorf("تعذر إغلاق ملف التحديث: %w", err)
	}

	// Remove old installer if present, then rename part file
	_ = os.Remove(targetPath)
	if err := os.Rename(partPath, targetPath); err != nil {
		return "", fmt.Errorf("تعذر نقل ملف التحديث إلى مساره النهائي: %w", err)
	}

	return targetPath, nil
}

package service

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	githubAPIBaseURL         = "https://api.github.com"
	maxReleaseResponseBytes  = 4 << 20
	maxChecksumsBytes        = 1 << 20
	maxUpdateBinaryBytes     = 512 << 20
	restartDelayEnvironment  = "NEW_API_RESTART_DELAY"
	restartBackupEnvironment = "NEW_API_RESTART_BACKUP"
)

type githubReleaseAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type githubRelease struct {
	TagName     string               `json:"tag_name"`
	Name        string               `json:"name"`
	Body        string               `json:"body"`
	HTMLURL     string               `json:"html_url"`
	PublishedAt string               `json:"published_at"`
	Prerelease  bool                 `json:"prerelease"`
	Draft       bool                 `json:"draft"`
	Assets      []githubReleaseAsset `json:"assets"`
}

type SystemUpdateInfo struct {
	Repository      string `json:"repository"`
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	CanUpdate       bool   `json:"can_update"`
	AssetName       string `json:"asset_name,omitempty"`
	AssetSize       int64  `json:"asset_size,omitempty"`
	Platform        string `json:"platform"`
	ReleaseURL      string `json:"release_url,omitempty"`
	ReleaseNotes    string `json:"release_notes,omitempty"`
	PublishedAt     string `json:"published_at,omitempty"`
	Prerelease      bool   `json:"prerelease"`
}

var systemUpdateMutex sync.Mutex

func NormalizeGitHubRepository(value string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(value), "/")
	if trimmed == "" {
		return "", errors.New("GitHub repository is required")
	}

	repositoryPath := trimmed
	if strings.HasPrefix(strings.ToLower(trimmed), "http://") || strings.HasPrefix(strings.ToLower(trimmed), "https://") {
		parsed, err := url.Parse(trimmed)
		if err != nil || !strings.EqualFold(parsed.Hostname(), "github.com") {
			return "", errors.New("GitHub repository URL must use github.com")
		}
		repositoryPath = strings.Trim(parsed.Path, "/")
	}

	parts := strings.Split(repositoryPath, "/")
	if len(parts) != 2 {
		return "", errors.New("GitHub repository must use owner/repository format")
	}

	owner := parts[0]
	repository := strings.TrimSuffix(parts[1], ".git")
	if !validGitHubRepositoryPart(owner) || !validGitHubRepositoryPart(repository) {
		return "", errors.New("GitHub repository contains invalid characters")
	}

	return owner + "/" + repository, nil
}

func validGitHubRepositoryPart(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func systemUpdateHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyFromEnvironment
	transport.ForceAttemptHTTP2 = true

	return &http.Client{
		Transport: transport,
		Timeout:   2 * time.Minute,
		CheckRedirect: func(request *http.Request, previous []*http.Request) error {
			if len(previous) >= 10 {
				return errors.New("too many redirects while downloading update")
			}
			return validateGitHubDownloadURL(request.URL)
		},
	}
}

func validateGitHubDownloadURL(downloadURL *url.URL) error {
	if downloadURL == nil || downloadURL.Scheme != "https" {
		return errors.New("update download URL must use HTTPS")
	}

	host := strings.ToLower(downloadURL.Hostname())
	if host == "github.com" || host == "api.github.com" || strings.HasSuffix(host, ".githubusercontent.com") {
		return nil
	}
	return fmt.Errorf("update download host is not allowed: %s", host)
}

func newGitHubRequest(ctx context.Context, requestURL string) (*http.Request, error) {
	parsed, err := url.Parse(requestURL)
	if err != nil {
		return nil, err
	}
	if err := validateGitHubDownloadURL(parsed); err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "new-api-system-updater")
	return request, nil
}

func fetchGitHubReleases(ctx context.Context, repository string) ([]githubRelease, error) {
	parts := strings.Split(repository, "/")
	requestURL := fmt.Sprintf(
		"%s/repos/%s/%s/releases?per_page=20",
		githubAPIBaseURL,
		url.PathEscape(parts[0]),
		url.PathEscape(parts[1]),
	)
	request, err := newGitHubRequest(ctx, requestURL)
	if err != nil {
		return nil, err
	}

	response, err := systemUpdateHTTPClient().Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to contact GitHub releases API: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub releases API returned status %d", response.StatusCode)
	}

	var releases []githubRelease
	if err := common.DecodeJson(io.LimitReader(response.Body, maxReleaseResponseBytes), &releases); err != nil {
		return nil, fmt.Errorf("failed to decode GitHub release response: %w", err)
	}
	return releases, nil
}

func updateAssetSuffix() string {
	suffix := "-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		suffix += ".exe"
	}
	return suffix
}

func selectSystemUpdateAssets(release githubRelease) (*githubReleaseAsset, *githubReleaseAsset) {
	var binaryAsset *githubReleaseAsset
	var checksumsAsset *githubReleaseAsset
	suffix := updateAssetSuffix()
	for index := range release.Assets {
		asset := &release.Assets[index]
		switch {
		case asset.Name == "checksums.txt":
			checksumsAsset = asset
		case strings.HasPrefix(asset.Name, "new-api-") && strings.HasSuffix(asset.Name, suffix):
			binaryAsset = asset
		}
	}
	return binaryAsset, checksumsAsset
}

func checkSystemUpdate(ctx context.Context, repository string) (*SystemUpdateInfo, *githubReleaseAsset, *githubReleaseAsset, error) {
	normalizedRepository, err := NormalizeGitHubRepository(repository)
	if err != nil {
		return nil, nil, nil, err
	}

	releases, err := fetchGitHubReleases(ctx, normalizedRepository)
	if err != nil {
		return nil, nil, nil, err
	}

	var latest *githubRelease
	for index := range releases {
		if !releases[index].Draft {
			latest = &releases[index]
			break
		}
	}
	if latest == nil {
		return nil, nil, nil, errors.New("no published releases found in the configured repository")
	}

	binaryAsset, checksumsAsset := selectSystemUpdateAssets(*latest)
	info := &SystemUpdateInfo{
		Repository:      normalizedRepository,
		CurrentVersion:  common.Version,
		LatestVersion:   latest.TagName,
		UpdateAvailable: latest.TagName != "" && latest.TagName != common.Version,
		CanUpdate:       binaryAsset != nil && checksumsAsset != nil,
		Platform:        runtime.GOOS + "/" + runtime.GOARCH,
		ReleaseURL:      latest.HTMLURL,
		ReleaseNotes:    latest.Body,
		PublishedAt:     latest.PublishedAt,
		Prerelease:      latest.Prerelease,
	}
	if binaryAsset != nil {
		info.AssetName = binaryAsset.Name
		info.AssetSize = binaryAsset.Size
	}
	return info, binaryAsset, checksumsAsset, nil
}

func CheckSystemUpdate(ctx context.Context, repository string) (*SystemUpdateInfo, error) {
	info, _, _, err := checkSystemUpdate(ctx, repository)
	return info, err
}

func downloadUpdateBytes(ctx context.Context, downloadURL string, maxBytes int64) ([]byte, error) {
	request, err := newGitHubRequest(ctx, downloadURL)
	if err != nil {
		return nil, err
	}
	response, err := systemUpdateHTTPClient().Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update download returned status %d", response.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, errors.New("update download exceeded the allowed size")
	}
	return data, nil
}

func checksumForAsset(checksums []byte, assetName string) (string, error) {
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		if filepath.Base(name) != assetName {
			continue
		}
		checksum := strings.ToLower(fields[0])
		decoded, err := hex.DecodeString(checksum)
		if err != nil || len(decoded) != sha256.Size {
			return "", errors.New("release checksum is invalid")
		}
		return checksum, nil
	}
	return "", fmt.Errorf("release checksum does not contain %s", assetName)
}

func currentExecutablePath() (string, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	resolvedPath, err := filepath.EvalSymlinks(executablePath)
	if err == nil {
		executablePath = resolvedPath
	}
	return filepath.Abs(executablePath)
}

func downloadUpdateBinary(ctx context.Context, asset githubReleaseAsset, targetPath string, expectedChecksum string) (string, error) {
	if asset.Size < 0 || asset.Size > maxUpdateBinaryBytes {
		return "", errors.New("update binary size is invalid")
	}
	request, err := newGitHubRequest(ctx, asset.BrowserDownloadURL)
	if err != nil {
		return "", err
	}
	response, err := systemUpdateHTTPClient().Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("binary download returned status %d", response.StatusCode)
	}

	pattern := ".new-api-update-*"
	if runtime.GOOS == "windows" {
		pattern += ".exe"
	}
	stagedFile, err := os.CreateTemp(filepath.Dir(targetPath), pattern)
	if err != nil {
		return "", fmt.Errorf("cannot create update file beside the executable: %w", err)
	}
	stagedPath := stagedFile.Name()
	removeStaged := true
	defer func() {
		_ = stagedFile.Close()
		if removeStaged {
			_ = os.Remove(stagedPath)
		}
	}()

	hash := sha256.New()
	limit := int64(maxUpdateBinaryBytes)
	if asset.Size > 0 {
		limit = asset.Size
	}
	written, err := io.Copy(io.MultiWriter(stagedFile, hash), io.LimitReader(response.Body, limit+1))
	if err != nil {
		return "", err
	}
	if written > limit || (asset.Size > 0 && written != asset.Size) {
		return "", errors.New("downloaded binary size does not match the release asset")
	}
	if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(hash.Sum(nil))), []byte(expectedChecksum)) != 1 {
		return "", errors.New("downloaded binary checksum verification failed")
	}
	if err := stagedFile.Sync(); err != nil {
		return "", err
	}
	if err := stagedFile.Close(); err != nil {
		return "", err
	}

	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		return "", err
	}
	mode := targetInfo.Mode().Perm()
	if mode&0111 == 0 {
		mode |= 0755
	}
	if err := os.Chmod(stagedPath, mode); err != nil {
		return "", err
	}

	removeStaged = false
	return stagedPath, nil
}

func binaryReportsVersion(ctx context.Context, binaryPath string, expectedVersion string) error {
	commandContext, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	output, err := exec.CommandContext(commandContext, binaryPath, "--version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("downloaded binary failed its version check: %w", err)
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.TrimSpace(line) == expectedVersion {
			return nil
		}
	}
	return fmt.Errorf("downloaded binary reported an unexpected version: %s", strings.TrimSpace(string(output)))
}

func installStagedBinary(stagedPath string, targetPath string) (string, error) {
	backupPath := targetPath + ".old"
	if err := os.Remove(backupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("cannot remove the previous update backup: %w", err)
	}
	if err := os.Rename(targetPath, backupPath); err != nil {
		return "", fmt.Errorf("cannot back up the current executable: %w", err)
	}
	if err := os.Rename(stagedPath, targetPath); err != nil {
		rollbackErr := os.Rename(backupPath, targetPath)
		if rollbackErr != nil {
			return "", fmt.Errorf("cannot install update: %v; rollback also failed: %w", err, rollbackErr)
		}
		return "", fmt.Errorf("cannot install update: %w", err)
	}
	return backupPath, nil
}

func rollbackInstalledBinary(targetPath string, backupPath string) error {
	failedPath := targetPath + ".failed-update"
	_ = os.Remove(failedPath)
	if err := os.Rename(targetPath, failedPath); err != nil {
		return err
	}
	if err := os.Rename(backupPath, targetPath); err != nil {
		_ = os.Rename(failedPath, targetPath)
		return err
	}
	_ = os.Remove(failedPath)
	return nil
}

func restartEnvironment(backupPath string) []string {
	environment := make([]string, 0, len(os.Environ())+2)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, restartDelayEnvironment+"=") || strings.HasPrefix(entry, restartBackupEnvironment+"=") {
			continue
		}
		environment = append(environment, entry)
	}
	return append(
		environment,
		restartDelayEnvironment+"=2s",
		restartBackupEnvironment+"="+backupPath,
	)
}

func scheduleUpdatedProcess(targetPath string, backupPath string) error {
	command := exec.Command(targetPath, os.Args[1:]...)
	command.Env = restartEnvironment(backupPath)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		return err
	}
	_ = command.Process.Release()

	go func() {
		time.Sleep(750 * time.Millisecond)
		os.Exit(0)
	}()
	return nil
}

func ApplySystemUpdate(ctx context.Context, repository string, expectedVersion string) (*SystemUpdateInfo, error) {
	systemUpdateMutex.Lock()
	defer systemUpdateMutex.Unlock()

	info, binaryAsset, checksumsAsset, err := checkSystemUpdate(ctx, repository)
	if err != nil {
		return nil, err
	}
	if expectedVersion == "" || info.LatestVersion != expectedVersion {
		return nil, errors.New("the selected release is no longer the latest version; check for updates again")
	}
	if !info.UpdateAvailable {
		return nil, errors.New("the current version is already up to date")
	}
	if binaryAsset == nil || checksumsAsset == nil {
		return nil, fmt.Errorf("release does not contain an update for %s", info.Platform)
	}

	checksums, err := downloadUpdateBytes(ctx, checksumsAsset.BrowserDownloadURL, maxChecksumsBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to download release checksums: %w", err)
	}
	expectedChecksum, err := checksumForAsset(checksums, binaryAsset.Name)
	if err != nil {
		return nil, err
	}

	targetPath, err := currentExecutablePath()
	if err != nil {
		return nil, fmt.Errorf("cannot locate the current executable: %w", err)
	}
	stagedPath, err := downloadUpdateBinary(ctx, *binaryAsset, targetPath, expectedChecksum)
	if err != nil {
		return nil, fmt.Errorf("failed to download update binary: %w", err)
	}
	removeStaged := true
	defer func() {
		if removeStaged {
			_ = os.Remove(stagedPath)
		}
	}()

	if err := binaryReportsVersion(ctx, stagedPath, info.LatestVersion); err != nil {
		return nil, err
	}
	backupPath, err := installStagedBinary(stagedPath, targetPath)
	if err != nil {
		return nil, err
	}
	removeStaged = false

	if err := scheduleUpdatedProcess(targetPath, backupPath); err != nil {
		if rollbackErr := rollbackInstalledBinary(targetPath, backupPath); rollbackErr != nil {
			return nil, fmt.Errorf("failed to restart after update: %v; rollback failed: %w", err, rollbackErr)
		}
		return nil, fmt.Errorf("failed to restart after update: %w", err)
	}
	return info, nil
}

func PrepareRestartedProcess() {
	delayValue := os.Getenv(restartDelayEnvironment)
	backupPath := os.Getenv(restartBackupEnvironment)
	_ = os.Unsetenv(restartDelayEnvironment)
	_ = os.Unsetenv(restartBackupEnvironment)

	if delay, err := time.ParseDuration(delayValue); err == nil && delay > 0 && delay <= 10*time.Second {
		time.Sleep(delay)
	}
	if backupPath == "" {
		return
	}

	executablePath, err := currentExecutablePath()
	if err != nil {
		return
	}
	if filepath.Clean(backupPath) == filepath.Clean(executablePath+".old") {
		_ = os.Remove(backupPath)
	}
}

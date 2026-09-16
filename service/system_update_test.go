package service

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeGitHubRepository(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{input: "owner/repository", want: "owner/repository", ok: true},
		{input: " https://github.com/owner/repository/ ", want: "owner/repository", ok: true},
		{input: "https://github.com/owner/repository.git", want: "owner/repository", ok: true},
		{input: "https://example.com/owner/repository", ok: false},
		{input: "owner/repository/releases", ok: false},
		{input: "owner/repo name", ok: false},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			actual, err := NormalizeGitHubRepository(test.input)
			if test.ok {
				require.NoError(t, err)
				require.Equal(t, test.want, actual)
				return
			}
			require.Error(t, err)
		})
	}
}

func TestChecksumForAsset(t *testing.T) {
	checksum := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	content := []byte(checksum + "  nachoai\n")

	actual, err := checksumForAsset(content, "nachoai")
	require.NoError(t, err)
	require.Equal(t, checksum, actual)

	_, err = checksumForAsset(content, "other-binary")
	require.Error(t, err)
}

func TestSelectSystemUpdateAssets(t *testing.T) {
	release := githubRelease{Assets: []githubReleaseAsset{
		{Name: "other-binary", Size: 1234},
		{Name: systemUpdateAssetName, Size: 1234},
		{Name: "checksums.txt", Size: 456},
	}}

	binaryAsset, checksumsAsset := selectSystemUpdateAssets(release)
	if systemUpdatePlatformSupported() {
		require.NotNil(t, binaryAsset)
		require.Equal(t, systemUpdateAssetName, binaryAsset.Name)
	} else {
		require.Nil(t, binaryAsset)
	}
	require.NotNil(t, checksumsAsset)
	require.Equal(t, "checksums.txt", checksumsAsset.Name)
}

func TestValidateGitHubDownloadURL(t *testing.T) {
	allowed := []string{
		"https://github.com/owner/repository/releases/download/v1/nachoai",
		"https://release-assets.githubusercontent.com/example",
	}
	for _, value := range allowed {
		parsed, err := url.Parse(value)
		require.NoError(t, err)
		require.NoError(t, validateGitHubDownloadURL(parsed))
	}

	blocked := []string{
		"http://github.com/owner/repository/releases/download/v1/nachoai",
		"https://github.com.example.test/update",
		"https://example.com/update",
	}
	for _, value := range blocked {
		parsed, err := url.Parse(value)
		require.NoError(t, err)
		require.Error(t, validateGitHubDownloadURL(parsed))
	}
}

func TestInstallAndRollbackStagedBinary(t *testing.T) {
	directory := t.TempDir()
	targetPath := filepath.Join(directory, "new-api")
	stagedPath := filepath.Join(directory, "new-api-update")
	require.NoError(t, os.WriteFile(targetPath, []byte("old"), 0755))
	require.NoError(t, os.WriteFile(stagedPath, []byte("new"), 0755))

	backupPath, err := installStagedBinary(stagedPath, targetPath)
	require.NoError(t, err)
	targetData, err := os.ReadFile(targetPath)
	require.NoError(t, err)
	require.Equal(t, "new", string(targetData))
	backupData, err := os.ReadFile(backupPath)
	require.NoError(t, err)
	require.Equal(t, "old", string(backupData))

	require.NoError(t, rollbackInstalledBinary(targetPath, backupPath))
	targetData, err = os.ReadFile(targetPath)
	require.NoError(t, err)
	require.Equal(t, "old", string(targetData))
}

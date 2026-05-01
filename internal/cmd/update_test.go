package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.0", 0},
		{"2.0.0", "1.9.9", 1},
		{"1.10.0", "1.9.0", 1},
		{"v1.0.0", "v1.0.1", -1},
		{"dev", "1.0.0", -1},
		{"1.0.0", "dev", 1},
		{"unknown", "1.0.0", -1},
		{"", "1.0.0", -1},
		{"1.2.3", "1.2.3", 0},
		{"1.2", "1.2.0", 0},
		{"1.2.0", "1.2", 0},
		{"10.0.0", "9.0.0", 1},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			got := compareVersions(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestBuildAssetName(t *testing.T) {
	origGOOS := updateGOOS
	origGOARCH := updateGOARCH
	defer func() { updateGOOS = origGOOS; updateGOARCH = origGOARCH }()

	tests := []struct {
		goos, goarch string
		want         string
	}{
		{"linux", "amd64", "typo-linux-amd64"},
		{"linux", "arm64", "typo-linux-arm64"},
		{"darwin", "amd64", "typo-darwin-amd64"},
		{"darwin", "arm64", "typo-darwin-arm64"},
		{"windows", "amd64", "typo-windows-amd64.exe"},
		{"windows", "arm64", "typo-windows-arm64.exe"},
	}

	for _, tt := range tests {
		t.Run(tt.goos+"_"+tt.goarch, func(t *testing.T) {
			updateGOOS = tt.goos
			updateGOARCH = tt.goarch
			got := buildAssetName()
			if got != tt.want {
				t.Errorf("buildAssetName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFindChecksumEntry(t *testing.T) {
	content := `abc123  typo-darwin-amd64
def456  typo-darwin-arm64
aaa111  typo-linux-amd64
bbb222  typo-linux-arm64
ccc333  typo-windows-amd64.exe
ddd444  typo-windows-arm64.exe
`

	tests := []struct {
		assetName string
		want      string
	}{
		{"typo-darwin-amd64", "abc123"},
		{"typo-darwin-arm64", "def456"},
		{"typo-linux-amd64", "aaa111"},
		{"typo-windows-amd64.exe", "ccc333"},
		{"typo-nonexistent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.assetName, func(t *testing.T) {
			got := findChecksumEntry(content, tt.assetName)
			if got != tt.want {
				t.Errorf("findChecksumEntry(%q) = %q, want %q", tt.assetName, got, tt.want)
			}
		})
	}
}

func TestSplitVersion(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"1.2.3", []int{1, 2, 3}},
		{"10.20.30", []int{10, 20, 30}},
		{"1", []int{1}},
		{"1.2", []int{1, 2}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitVersion(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("splitVersion(%q) len = %d, want %d", tt.input, len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitVersion(%q)[%d] = %d, want %d", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestIsHomebrewInstall(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("regular install", func(t *testing.T) {
		path := filepath.Join(tmpDir, "typo")
		if isHomebrewInstall(path) {
			t.Error("regular path should not be detected as homebrew")
		}
	})

	t.Run("homebrew install", func(t *testing.T) {
		cellar := filepath.Join("/opt/homebrew", "Cellar", "typo", "bin", "typo")
		if !isHomebrewInstall(cellar) {
			t.Error("homebrew path should be detected")
		}
	})
}

func TestFindAssetURLs(t *testing.T) {
	release := &githubRelease{
		TagName: "v1.0.0",
		Assets: []githubAsset{
			{
				Name:               "typo-linux-amd64",
				URL:                "https://api.github.com/repos/yuluo-yx/typo/releases/assets/1",
				BrowserDownloadURL: "https://github.com/yuluo-yx/typo/releases/download/v1.0.0/typo-linux-amd64",
			},
			{
				Name:               "checksums.txt",
				URL:                "https://api.github.com/repos/yuluo-yx/typo/releases/assets/2",
				BrowserDownloadURL: "https://github.com/yuluo-yx/typo/releases/download/v1.0.0/checksums.txt",
			},
		},
	}

	apiURL, cdnURL, checksumAPI := findAssetURLs(release, "typo-linux-amd64")
	if apiURL != "https://api.github.com/repos/yuluo-yx/typo/releases/assets/1" {
		t.Errorf("apiURL = %q", apiURL)
	}
	if cdnURL != "https://github.com/yuluo-yx/typo/releases/download/v1.0.0/typo-linux-amd64" {
		t.Errorf("cdnURL = %q", cdnURL)
	}
	if checksumAPI != "https://api.github.com/repos/yuluo-yx/typo/releases/assets/2" {
		t.Errorf("checksumAPI = %q", checksumAPI)
	}

	apiURL, _, _ = findAssetURLs(release, "typo-nonexistent")
	if apiURL != "" {
		t.Errorf("expected empty apiURL for nonexistent asset, got %q", apiURL)
	}
}

func TestVerifyChecksum(t *testing.T) {
	tmpDir := t.TempDir()

	binaryPath := filepath.Join(tmpDir, "typo-test")
	if err := os.WriteFile(binaryPath, []byte("hello world\n"), 0644); err != nil {
		t.Fatal(err)
	}

	checksumsPath := filepath.Join(tmpDir, "checksums.txt")
	// SHA-256 of "hello world\n".
	checksumsContent := "a948904f2f0f479b8f8197694b30184b0d2ed1c1cd2a1ec0fb85d299a192a447  typo-test\n"
	if err := os.WriteFile(checksumsPath, []byte(checksumsContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := verifyChecksum(binaryPath, checksumsPath, "typo-test"); err != nil {
		t.Errorf("valid checksum should pass: %v", err)
	}

	if err := os.WriteFile(checksumsPath, []byte("0000000000000000000000000000000000000000000000000000000000000000  typo-test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(binaryPath, checksumsPath, "typo-test"); err == nil {
		t.Error("tampered checksum should fail")
	}
}

func TestReplaceBinary(t *testing.T) {
	tmpDir := t.TempDir()

	current := filepath.Join(tmpDir, "current-typo")
	newBin := filepath.Join(tmpDir, "new-typo")

	if err := os.WriteFile(current, []byte("old binary"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newBin, []byte("new binary"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := replaceBinary(newBin, current); err != nil {
		t.Fatalf("replaceBinary failed: %v", err)
	}

	data, err := os.ReadFile(current)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new binary" {
		t.Errorf("binary not replaced, got %q", string(data))
	}

	if _, err := os.Stat(current + ".old"); !os.IsNotExist(err) {
		t.Error("backup file should be removed after successful replace")
	}
}

func TestFetchReleaseByTag(t *testing.T) {
	origGet := updateHTTPGet
	origSleep := updateSleep
	defer func() { updateHTTPGet = origGet; updateSleep = origSleep }()

	t.Run("rate limited with retry", func(t *testing.T) {
		callCount := 0
		updateHTTPGet = func(req *http.Request) (*http.Response, error) {
			callCount++
			if callCount == 1 {
				return &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Header: http.Header{
						"X-Ratelimit-Reset": []string{fmt.Sprintf("%d", time.Now().Unix()+1)},
					},
					Body: io.NopCloser(strings.NewReader("")),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"tag_name":"v1.0.0","assets":[]}`)),
			}, nil
		}

		release, err := fetchReleaseByTag("latest")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if release.TagName != "v1.0.0" {
			t.Errorf("tag = %q, want v1.0.0", release.TagName)
		}
		if callCount != 2 {
			t.Errorf("expected 2 calls, got %d", callCount)
		}
	})

	t.Run("not found", func(t *testing.T) {
		updateHTTPGet = func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}

		_, err := fetchReleaseByTag("v9.9.9")
		if err == nil {
			t.Error("expected error for not found")
		}
	})
}

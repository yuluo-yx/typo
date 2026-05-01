package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Injectable for testing.
var (
	updateHTTPGet      func(req *http.Request) (*http.Response, error)
	updateHTTPDownload func(req *http.Request) (*http.Response, error)
	updateCopyFile     func(dst io.Writer, src io.Reader) (int64, error)
	updateSleep        func(d time.Duration)
	updateCachedLatest *githubRelease
	updateGOOS         = runtime.GOOS
	updateGOARCH       = runtime.GOARCH
)

// errStop is a sentinel that signals the chain to stop without printing an error.
var errStop = errors.New("stop")

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	URL                string `json:"url"`
}

// updateContext carries state through the handler chain.
type updateContext struct {
	args []string

	flags      updateFlags
	currentVer string
	targetTag  string

	latest       *githubRelease
	assetName    string
	apiURL       string
	cdnURL       string
	checksumsURL string

	exeDir     string
	binaryPath string
	currentExe string

	githubToken string
	installed   bool
}

type updateFlags struct {
	checkOnly bool
	force     bool
	dryRun    bool
	targetVer string
}

// updateHandler is a single step in the update pipeline.
type updateHandler interface {
	Handle(ctx *updateContext) error
}

// updateHandlerFunc adapts a function to the updateHandler interface.
type updateHandlerFunc func(ctx *updateContext) error

func (f updateHandlerFunc) Handle(ctx *updateContext) error { return f(ctx) }

func init() {
	updateSleep = time.Sleep
	apiClient := &http.Client{Timeout: 30 * time.Second}
	downloadClient := &http.Client{Timeout: 10 * time.Minute}
	updateHTTPGet = apiClient.Do
	updateHTTPDownload = downloadClient.Do
	updateCopyFile = func(dst io.Writer, src io.Reader) (int64, error) {
		return io.Copy(dst, src)
	}
}
// update chain 
func cmdUpdate(args []string) int {
	ctx := &updateContext{args: args}

	if code := runChain(ctx,
		updateHandlerFunc(flagParseHandler),
		updateHandlerFunc(versionCheckHandler),
		updateHandlerFunc(assetResolveHandler),
		updateHandlerFunc(downloadHandler),
		updateHandlerFunc(checksumHandler),
		updateHandlerFunc(installHandler),
	); code != 0 {
		return code
	}

	if ctx.installed {
		fmt.Printf("Updated typo to %s\n", ctx.targetTag)
	}
	return 0
}

// runChain executes handlers in order. It returns 0 on success or sentinel stop,
// or 1 on error (after printing to stderr).
func runChain(ctx *updateContext, handlers ...updateHandler) int {
	for _, h := range handlers {
		err := h.Handle(ctx)
		if err == nil {
			continue
		}
		if errors.Is(err, errStop) {
			return 0
		}
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

// --- handler implementations ---

func flagParseHandler(ctx *updateContext) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.BoolVar(&ctx.flags.checkOnly, "check", false, "only check for updates, do not install")
	fs.BoolVar(&ctx.flags.force, "force", false, "reinstall even if versions match")
	fs.BoolVar(&ctx.flags.dryRun, "dry-run", false, "simulate without making changes")
	fs.StringVar(&ctx.flags.targetVer, "version", "", "install a specific version (e.g. v1.2.0)")

	if err := fs.Parse(ctx.args); err != nil {
		return err
	}
	return nil
}

func versionCheckHandler(ctx *updateContext) error {
	ctx.currentVer, _, _ = resolveVersionInfo()

	latest, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}
	ctx.latest = latest

	ctx.targetTag = latest.TagName
	if ctx.flags.targetVer != "" {
		ctx.targetTag = ctx.flags.targetVer
	}

	if ctx.flags.checkOnly {
		if ctx.flags.force || compareVersions(ctx.currentVer, strings.TrimPrefix(ctx.targetTag, "v")) < 0 {
			fmt.Printf("Update available: %s → %s\n", ctx.currentVer, ctx.targetTag)
		} else {
			fmt.Printf("typo is up to date (%s)\n", ctx.currentVer)
		}
		return errStop
	}

	if !ctx.flags.force && ctx.flags.targetVer == "" {
		if compareVersions(ctx.currentVer, strings.TrimPrefix(ctx.targetTag, "v")) >= 0 {
			fmt.Printf("typo is up to date (%s)\n", ctx.currentVer)
			return errStop
		}
	}

	return nil
}

func assetResolveHandler(ctx *updateContext) error {
	ctx.assetName = buildAssetName()

	var release *githubRelease
	if ctx.flags.targetVer != "" {
		r, err := fetchReleaseByTag(ctx.targetTag)
		if err != nil {
			return fmt.Errorf("failed to fetch release %s: %w", ctx.targetTag, err)
		}
		release = r
	} else {
		release = ctx.latest
	}

	ctx.apiURL, ctx.cdnURL, ctx.checksumsURL = findAssetURLs(release, ctx.assetName)
	if ctx.apiURL == "" {
		return fmt.Errorf("no binary found for %s/%s", updateGOOS, updateGOARCH)
	}

	if ctx.flags.dryRun {
		fmt.Printf("Would download %s\n", ctx.assetName)
		fmt.Printf("Current: %s → Target: %s\n", ctx.currentVer, ctx.targetTag)
		return errStop
	}

	return nil
}

func downloadHandler(ctx *updateContext) error {
	currentExe, err := executable()
	if err != nil {
		return fmt.Errorf("cannot determine current executable path: %w", err)
	}
	ctx.currentExe = currentExe
	ctx.exeDir = filepath.Dir(currentExe)
	ctx.binaryPath = filepath.Join(ctx.exeDir, ctx.assetName+".new")
	ctx.githubToken = os.Getenv("GITHUB_TOKEN")

	fmt.Printf("> DOWNLOADING UPDATE...\n")
	fmt.Printf("  version : %s\n", ctx.targetTag)
	fmt.Printf("  asset   : %s\n", ctx.assetName)
	fmt.Printf("  source  : %s\n", downloadSourceLabel(ctx.cdnURL))
	fmt.Println("  ─────────────────────────────────────────")

	if err := downloadBinary(ctx.binaryPath, ctx.apiURL, ctx.cdnURL, ctx.githubToken); err != nil {
		return fmt.Errorf("download failed: %w\n\n"+
			"Tip: You can manually download from:\n"+
			"  https://github.com/yuluo-yx/typo/releases/tag/%s\n"+
			"  Replace your current typo binary with the downloaded file.",
			err, ctx.targetTag)
	}

	return nil
}

func downloadSourceLabel(cdnURL string) string {
	if cdnURL != "" {
		return "CDN (api fallback)"
	}
	return "API"
}

func checksumHandler(ctx *updateContext) error {
	if ctx.checksumsURL == "" {
		return nil
	}

	checksumsPath := filepath.Join(ctx.exeDir, "checksums.txt")
	if err := downloadToFile(checksumsPath, ctx.checksumsURL, true, false, ctx.githubToken); err != nil {
		return fmt.Errorf("checksums download failed: %w", err)
	}
	defer func() { _ = os.Remove(checksumsPath) }()

	if err := verifyChecksum(ctx.binaryPath, checksumsPath, ctx.assetName); err != nil {
		return fmt.Errorf("checksum verification failed: %w", err)
	}

	return nil
}

func installHandler(ctx *updateContext) error {
	if isHomebrewInstall(ctx.currentExe) {
		_ = os.Remove(ctx.binaryPath)
		return fmt.Errorf("typo was installed via Homebrew. Use 'brew upgrade typo' instead.")
	}

	if err := replaceBinary(ctx.binaryPath, ctx.currentExe); err != nil {
		_ = os.Remove(ctx.binaryPath)
		return err
	}

	ctx.installed = true
	return nil
}

// --- release fetching ---

func fetchLatestRelease() (*githubRelease, error) {
	if updateCachedLatest != nil {
		return updateCachedLatest, nil
	}
	return fetchReleaseByTag("latest")
}

func fetchReleaseByTag(tag string) (*githubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/yuluo-yx/typo/releases/%s", tag)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "typo")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := updateHTTPGet(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
			if ts, err := strconv.ParseInt(reset, 10, 64); err == nil {
				wait := time.Until(time.Unix(ts, 0)) + time.Second
				if wait > 0 && wait < 15*time.Minute {
					updateSleep(wait)
					return fetchReleaseByTag(tag)
				}
			}
		}
		return nil, fmt.Errorf("GitHub API rate limited, try again later or set GITHUB_TOKEN")
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("release %s not found", tag)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

// --- asset helpers ---

func buildAssetName() string {
	switch updateGOOS {
	case "windows":
		return fmt.Sprintf("typo-windows-%s.exe", updateGOARCH)
	default:
		return fmt.Sprintf("typo-%s-%s", updateGOOS, updateGOARCH)
	}
}

func findAssetURLs(release *githubRelease, assetName string) (apiURL, cdnURL, checksumAPI string) {
	for _, a := range release.Assets {
		if a.Name == assetName {
			apiURL = a.URL
			cdnURL = a.BrowserDownloadURL
		}
		if a.Name == "checksums.txt" {
			checksumAPI = a.URL
		}
	}
	return
}

// --- download ---

func downloadBinary(path, apiURL, cdnURL, token string) error {
	if cdnURL != "" {
		if err := downloadWithTimeout(path, cdnURL, false, false, token, 5*time.Second); err == nil {
			fmt.Fprint(os.Stderr, "\n")
			return nil
		}
		fmt.Fprintf(os.Stderr, "  CDN timed out, switching to API...\n")
	}
	return downloadToFile(path, apiURL, true, true, token)
}

func downloadToFile(path, url string, isAPI bool, showProgress bool, token string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "typo")
	if isAPI {
		req.Header.Set("Accept", "application/octet-stream")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := updateHTTPDownload(req)
	if err != nil {
		return fmt.Errorf("network error: %w (check your connection or proxy settings)", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	if showProgress {
		bar := &progressWriter{
			total:     resp.ContentLength,
			start:     time.Now(),
			lastPrint: time.Now(),
		}
		_, err = updateCopyFile(f, io.TeeReader(resp.Body, bar))
		if bar.written > 0 {
			fmt.Fprint(os.Stderr, "\n")
		}
	} else {
		_, err = updateCopyFile(f, resp.Body)
	}
	return err
}

func downloadWithTimeout(path, url string, isAPI bool, showProgress bool, token string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ch := make(chan error, 1)
	go func() {
		ch <- downloadToFile(path, url, isAPI, showProgress, token)
	}()

	select {
	case err := <-ch:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// --- progress ---

const barWidth = 36

var spinnerFrames = []string{"◐", "◓", "◑", "◒"}

type progressWriter struct {
	total     int64
	written   int64
	start     time.Time
	lastPrint time.Time
	spin      int
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n := len(b)
	p.written += int64(n)
	if p.written == 0 {
		return n, nil
	}
	now := time.Now()
	if now.Sub(p.lastPrint) < 80*time.Millisecond {
		return n, nil
	}
	p.lastPrint = now

	elapsed := now.Sub(p.start)
	mbps := float64(p.written) / elapsed.Seconds() / (1024 * 1024)
	mbWritten := float64(p.written) / (1024 * 1024)

	spin := spinnerFrames[p.spin%len(spinnerFrames)]
	p.spin++

	if p.total > 0 {
		pct := float64(p.written) / float64(p.total)
		filled := int(pct * barWidth)
		bar := strings.Repeat("█", filled) + strings.Repeat("▒", barWidth-filled)
		mbTotal := float64(p.total) / (1024 * 1024)

		var eta string
		if mbps > 0.01 {
			sec := int(float64(p.total-p.written) / (mbps * 1024 * 1024))
			eta = fmt.Sprintf("ETA %ds", sec)
		} else {
			eta = "ETA --s"
		}

		fmt.Fprintf(os.Stderr, "\r  %s [%s] %3.0f%%  %.1f/%.1fMB  %.1fMB/s  %s  ",
			spin, bar, pct*100, mbWritten, mbTotal, mbps, eta)
	} else {
		fmt.Fprintf(os.Stderr, "\r  %s %.1fMB  %.1fMB/s    ", spin, mbWritten, mbps)
	}
	return n, nil
}

// --- checksum ---

func verifyChecksum(binaryPath, checksumsPath, assetName string) error {
	data, err := os.ReadFile(checksumsPath)
	if err != nil {
		return err
	}

	expected := findChecksumEntry(string(data), assetName)
	if expected == "" {
		return fmt.Errorf("no checksum entry for %s", assetName)
	}

	f, err := os.Open(binaryPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	actual := fmt.Sprintf("%x", h.Sum(nil))

	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("checksum mismatch\n  expected: %s\n    actual: %s", expected, actual)
	}

	return nil
}

func findChecksumEntry(checksumsContent, assetName string) string {
	for _, line := range strings.Split(checksumsContent, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == assetName {
			return strings.ToLower(fields[0])
		}
	}
	return ""
}

// --- install ---

var homebrewCellarPrefixes = []string{
	"/home/linuxbrew/.linuxbrew/",
	"/opt/homebrew/",
	"/usr/local/Homebrew/",
	"/usr/local/Cellar/",
}

func isHomebrewInstall(exePath string) bool {
	realPath, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		return false
	}
	for _, prefix := range homebrewCellarPrefixes {
		if strings.HasPrefix(realPath, prefix) {
			return true
		}
	}
	return false
}

func replaceBinary(newPath, currentExe string) error {
	fi, err := os.Stat(currentExe)
	if err != nil {
		return fmt.Errorf("cannot stat current binary: %w", err)
	}

	if err := os.Chmod(newPath, fi.Mode()|0111); err != nil {
		return fmt.Errorf("cannot make new binary executable: %w", err)
	}

	backupPath := currentExe + ".old"
	_ = os.Remove(backupPath)
	if err := os.Rename(currentExe, backupPath); err != nil {
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied: cannot replace %s (try with sudo)", currentExe)
		}
		return fmt.Errorf("cannot backup current binary: %w", err)
	}

	if err := os.Rename(newPath, currentExe); err != nil {
		_ = os.Rename(backupPath, currentExe)
		return fmt.Errorf("cannot install new binary: %w", err)
	}

	_ = os.Remove(backupPath)
	return nil
}

// --- version comparison ---

func compareVersions(a, b string) int {
	a = strings.TrimPrefix(strings.TrimSpace(a), "v")
	b = strings.TrimPrefix(strings.TrimSpace(b), "v")

	if a == "dev" || a == "unknown" || a == "" {
		return -1
	}
	if b == "dev" || b == "unknown" || b == "" {
		return 1
	}

	aParts := splitVersion(a)
	bParts := splitVersion(b)

	for i := range max(len(aParts), len(bParts)) {
		aNum := partAt(aParts, i)
		bNum := partAt(bParts, i)
		if aNum < bNum {
			return -1
		}
		if aNum > bNum {
			return 1
		}
	}

	return 0
}

func splitVersion(v string) []int {
	parts := strings.Split(v, ".")
	nums := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			n = 0
		}
		nums[i] = n
	}
	return nums
}

func partAt(parts []int, i int) int {
	if i >= len(parts) {
		return 0
	}
	return parts[i]
}

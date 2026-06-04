package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	upgradeRepo   = "vanducng/miu-db"
	upgradeBinary = "miudb"
)

func upgradeCommand(opts *options) *cobra.Command {
	var checkOnly bool
	var target string
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade miudb to the latest release",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			return runUpgrade(ctx, cmd.OutOrStdout(), checkOnly, target)
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check", false, "Only report whether a newer version exists; do not install")
	cmd.Flags().StringVar(&target, "version", "", "Install a specific version tag (e.g. v0.2.4) instead of the latest")
	return cmd
}

func runUpgrade(ctx context.Context, w io.Writer, checkOnly bool, target string) error {
	current := versionString()
	client := &http.Client{Timeout: 5 * time.Minute}

	latest := strings.TrimSpace(target)
	if latest == "" {
		tag, err := latestReleaseTag(ctx, client)
		if err != nil {
			return err
		}
		latest = tag
	}
	updateAvailable := normalizeTag(current) != normalizeTag(latest)

	if checkOnly {
		data := map[string]any{"current": current, "latest": latest, "update_available": updateAvailable}
		return writeJSON(w, Envelope{OK: true, Kind: "upgrade.checked", Command: "upgrade", Summary: data, Data: data})
	}
	if target == "" && !updateAvailable {
		data := map[string]any{"current": current, "latest": latest}
		return writeJSON(w, Envelope{OK: true, Kind: "upgrade.up_to_date", Command: "upgrade", Summary: data, Data: data})
	}

	asset := upgradeAssetName(runtime.GOOS, runtime.GOARCH)
	if asset == "" {
		return &CLIError{Code: "upgrade.unsupported_platform", Message: fmt.Sprintf("unsupported platform %s/%s", runtime.GOOS, runtime.GOARCH), Exit: 2}
	}
	base := fmt.Sprintf("https://github.com/%s/releases/download/%s", upgradeRepo, latest)

	archive, err := download(ctx, client, base+"/"+asset)
	if err != nil {
		return &CLIError{Code: "upgrade.download_failed", Message: fmt.Sprintf("failed to download %s: %v", asset, err), Hint: fmt.Sprintf("check that release %s exists", latest), Exit: 1}
	}
	if sums, err := download(ctx, client, base+"/checksums.txt"); err == nil {
		if want := checksumFor(string(sums), asset); want != "" {
			if got := fmt.Sprintf("%x", sha256.Sum256(archive)); got != want {
				return &CLIError{Code: "upgrade.checksum_mismatch", Message: fmt.Sprintf("checksum mismatch for %s", asset), Exit: 1}
			}
		}
	}
	bin, err := extractBinary(archive, runtime.GOOS)
	if err != nil {
		return &CLIError{Code: "upgrade.extract_failed", Message: err.Error(), Exit: 1}
	}
	installedPath, err := replaceExecutable(bin)
	if err != nil {
		return err
	}

	data := map[string]any{"from": current, "to": latest, "path": installedPath}
	return writeJSON(w, Envelope{OK: true, Kind: "upgrade.applied", Command: "upgrade", Summary: data, Data: data})
}

func latestReleaseTag(ctx context.Context, client *http.Client) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", upgradeRepo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "miudb-upgrade")
	resp, err := client.Do(req)
	if err != nil {
		return "", &CLIError{Code: "upgrade.check_failed", Message: fmt.Sprintf("cannot reach GitHub: %v", err), Retry: true, Exit: 1}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", &CLIError{Code: "upgrade.check_failed", Message: fmt.Sprintf("GitHub returned status %d", resp.StatusCode), Exit: 1}
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.TagName == "" {
		return "", &CLIError{Code: "upgrade.check_failed", Message: "GitHub response had no tag_name", Exit: 1}
	}
	return payload.TagName, nil
}

func download(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "miudb-upgrade")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func upgradeAssetName(goos, goarch string) string {
	var arch string
	switch goarch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "arm64"
	default:
		return ""
	}
	switch goos {
	case "darwin", "linux":
		return fmt.Sprintf("%s_%s_%s.tar.gz", upgradeBinary, goos, arch)
	case "windows":
		return fmt.Sprintf("%s_%s_%s.zip", upgradeBinary, goos, arch)
	default:
		return ""
	}
}

func binaryName(goos string) string {
	if goos == "windows" {
		return upgradeBinary + ".exe"
	}
	return upgradeBinary
}

func checksumFor(checksums, asset string) string {
	for _, line := range strings.Split(checksums, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == asset {
			return strings.ToLower(fields[0])
		}
	}
	return ""
}

func normalizeTag(tag string) string {
	return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(tag)), "v")
}

func extractBinary(archive []byte, goos string) ([]byte, error) {
	want := binaryName(goos)
	if goos == "windows" {
		zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, err
		}
		for _, f := range zr.File {
			if path.Base(f.Name) == want {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return io.ReadAll(rc)
			}
		}
		return nil, fmt.Errorf("%s not found in archive", want)
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if path.Base(hdr.Name) == want {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("%s not found in archive", want)
}

// replaceExecutable atomically swaps the running binary for newBin and returns its path.
func replaceExecutable(newBin []byte) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	if strings.Contains(exe, "/Cellar/") || strings.Contains(exe, "/homebrew/") {
		return exe, &CLIError{Code: "upgrade.managed_install", Message: "miudb was installed via Homebrew", Hint: "upgrade with: brew upgrade miudb", Exit: 2}
	}

	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".miudb-upgrade-*")
	if err != nil {
		return exe, permHint(err, dir)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(newBin); err != nil {
		tmp.Close()
		return exe, err
	}
	if err := tmp.Close(); err != nil {
		return exe, err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return exe, err
	}

	if runtime.GOOS == "windows" {
		old := exe + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			return exe, permHint(err, dir)
		}
		if err := os.Rename(tmpName, exe); err != nil {
			_ = os.Rename(old, exe)
			return exe, err
		}
		_ = os.Remove(old)
		return exe, nil
	}
	if err := os.Rename(tmpName, exe); err != nil {
		return exe, permHint(err, dir)
	}
	return exe, nil
}

func permHint(err error, dir string) error {
	if os.IsPermission(err) {
		return &CLIError{Code: "upgrade.permission_denied", Message: fmt.Sprintf("cannot write to %s: %v", dir, err), Hint: "re-run with elevated permissions (e.g. sudo) or reinstall via your package manager", Exit: 2}
	}
	return err
}

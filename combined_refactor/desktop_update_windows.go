//go:build windows

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const maxDesktopUpdateBytes int64 = 128 << 20

func stageDesktopUpdate(ctx context.Context, asset releaseAsset, report func(string)) error {
	expectedDigest, err := normalizeSHA256(asset.Digest)
	if err != nil {
		return err
	}
	if asset.Size > maxDesktopUpdateBytes {
		return fmt.Errorf("update asset exceeds the %d MiB limit", maxDesktopUpdateBytes>>20)
	}
	if err := validateDesktopUpdateURL(asset.BrowserDownloadURL); err != nil {
		return err
	}

	target, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate current executable: %w", err)
	}
	if !strings.EqualFold(filepath.Ext(target), ".exe") {
		return fmt.Errorf("current executable is not a Windows EXE")
	}

	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		cacheRoot = os.TempDir()
	}
	cacheRoot = filepath.Join(cacheRoot, "CFData", "updates")
	if err := os.MkdirAll(cacheRoot, 0700); err != nil {
		return fmt.Errorf("create update cache: %w", err)
	}
	workDir, err := os.MkdirTemp(cacheRoot, "pending-")
	if err != nil {
		return fmt.Errorf("create update work directory: %w", err)
	}

	updatePath := filepath.Join(workDir, asset.Name)
	report("正在下载更新包")
	if err := downloadDesktopUpdateAsset(ctx, asset, expectedDigest, updatePath); err != nil {
		return err
	}

	updaterPath := filepath.Join(workDir, "CFData-Updater.exe")
	if err := copyUpdateFile(target, updaterPath); err != nil {
		return fmt.Errorf("prepare update helper: %w", err)
	}
	report("更新包校验完成，正在准备重启")
	cmd := exec.Command(updaterPath,
		"--cfdata-apply-update",
		"--cfdata-update-source="+updatePath,
		"--cfdata-update-target="+target,
		"--cfdata-update-sha256="+expectedDigest,
	)
	cmd.Dir = workDir
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start update helper: %w", err)
	}
	return nil
}

func validateDesktopUpdateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Host, "github.com") || parsed.User != nil {
		return fmt.Errorf("desktop update URL must be an HTTPS GitHub release asset")
	}
	return nil
}

func downloadDesktopUpdateAsset(ctx context.Context, asset releaseAsset, expectedDigest, destination string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return fmt.Errorf("create update download request: %w", err)
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", "CFData-Desktop/"+appVersion)
	client := *upstreamHTTPClient
	client.Timeout = 10 * time.Minute
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download update asset: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("download update asset: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if resp.ContentLength > maxDesktopUpdateBytes {
		return fmt.Errorf("update response exceeds the %d MiB limit", maxDesktopUpdateBytes>>20)
	}

	partialPath := destination + ".part"
	file, err := os.OpenFile(partialPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("create update file: %w", err)
	}
	defer func() {
		_ = file.Close()
		_ = os.Remove(partialPath)
	}()

	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(resp.Body, maxDesktopUpdateBytes+1))
	if err != nil {
		return fmt.Errorf("write update file: %w", err)
	}
	if written > maxDesktopUpdateBytes {
		return fmt.Errorf("update asset exceeds the %d MiB limit", maxDesktopUpdateBytes>>20)
	}
	if asset.Size > 0 && written != asset.Size {
		return fmt.Errorf("update asset size mismatch: got %d bytes, expected %d", written, asset.Size)
	}
	actualDigest := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actualDigest, expectedDigest) {
		return fmt.Errorf("update SHA-256 mismatch")
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync update file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close update file: %w", err)
	}
	if err := os.Rename(partialPath, destination); err != nil {
		return fmt.Errorf("finalize update file: %w", err)
	}
	return nil
}

func copyUpdateFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	if err := output.Sync(); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

func runDesktopUpdateApplierIfRequested(args []string) bool {
	requested := false
	for _, arg := range args {
		if arg == "--cfdata-apply-update" || strings.HasPrefix(arg, "--cfdata-apply-update=") {
			requested = true
			break
		}
	}
	if !requested {
		return false
	}

	flags := flag.NewFlagSet("cfdata-update", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	apply := flags.Bool("cfdata-apply-update", false, "")
	source := flags.String("cfdata-update-source", "", "")
	target := flags.String("cfdata-update-target", "", "")
	digest := flags.String("cfdata-update-sha256", "", "")
	if err := flags.Parse(args); err != nil || !*apply || *source == "" || *target == "" || *digest == "" {
		showDesktopError("CFData 更新", "更新参数无效，程序未被替换。")
		return true
	}
	if err := applyDesktopUpdate(*source, *target, *digest); err != nil {
		showDesktopError("CFData 更新失败", err.Error())
	}
	return true
}

func applyDesktopUpdate(source, target, expectedDigest string) error {
	expectedDigest, err := normalizeSHA256(expectedDigest)
	if err != nil {
		return err
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return fmt.Errorf("resolve update source: %w", err)
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolve update target: %w", err)
	}
	if strings.EqualFold(source, target) {
		return fmt.Errorf("update source and target must differ")
	}
	if !strings.EqualFold(filepath.Ext(source), ".exe") || !strings.EqualFold(filepath.Ext(target), ".exe") {
		return fmt.Errorf("update source and target must be EXE files")
	}
	if err := verifyUpdateFileSHA256(source, expectedDigest); err != nil {
		return err
	}

	deadline := time.Now().Add(60 * time.Second)
	var replaceErr error
	for time.Now().Before(deadline) {
		if err := replaceUpdateTarget(source, target); err == nil {
			cmd := exec.Command(target)
			cmd.Dir = filepath.Dir(target)
			if err := cmd.Start(); err != nil {
				return fmt.Errorf("start updated application: %w", err)
			}
			return nil
		} else {
			replaceErr = err
			time.Sleep(300 * time.Millisecond)
		}
	}
	return fmt.Errorf("replace current executable after waiting for it to close: %w", replaceErr)
}

func verifyUpdateFileSHA256(path, expectedDigest string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open update file: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	written, err := io.Copy(hash, io.LimitReader(file, maxDesktopUpdateBytes+1))
	if err != nil {
		return fmt.Errorf("hash update file: %w", err)
	}
	if written > maxDesktopUpdateBytes {
		return fmt.Errorf("update asset exceeds the %d MiB limit", maxDesktopUpdateBytes>>20)
	}
	actualDigest := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actualDigest, expectedDigest) {
		return fmt.Errorf("update SHA-256 mismatch")
	}
	return nil
}

func replaceUpdateTarget(source, target string) error {
	sourcePtr, err := syscall.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	targetPtr, err := syscall.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	const moveFileReplaceExisting = 0x1
	const moveFileWriteThrough = 0x8
	moveFileEx := syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")
	ok, _, callErr := moveFileEx.Call(
		uintptr(unsafe.Pointer(sourcePtr)),
		uintptr(unsafe.Pointer(targetPtr)),
		moveFileReplaceExisting|moveFileWriteThrough,
	)
	if ok == 0 {
		return fmt.Errorf("replace executable: %w", callErr)
	}
	return nil
}

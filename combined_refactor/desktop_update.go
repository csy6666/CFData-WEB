package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const desktopUpdateAssetPrefix = "cfdata-desktop-windows-"

var desktopUpdateState struct {
	sync.Mutex
	shutdown     func()
	shutdownOnce sync.Once
	updating     bool
}

func setDesktopUpdateShutdown(shutdown func()) {
	desktopUpdateState.Lock()
	desktopUpdateState.shutdown = shutdown
	desktopUpdateState.Unlock()
}

func requestDesktopUpdateShutdown() {
	desktopUpdateState.shutdownOnce.Do(func() {
		go func() {
			time.Sleep(1200 * time.Millisecond)
			desktopUpdateState.Lock()
			shutdown := desktopUpdateState.shutdown
			desktopUpdateState.Unlock()
			if shutdown != nil {
				shutdown()
			}
		}()
	})
}

func desktopSelfUpdateEnabled() bool {
	return desktopBuild == "true" && runtime.GOOS == "windows"
}

func desktopUpdateAssetName(arch string) string {
	return desktopUpdateAssetPrefix + arch + ".exe"
}

func desktopRuntimeDirectory(executablePath string) (string, error) {
	executablePath = strings.TrimSpace(executablePath)
	if executablePath == "" {
		return "", fmt.Errorf("desktop executable path is empty")
	}
	directory := filepath.Dir(executablePath)
	if directory == "" || directory == "." {
		return "", fmt.Errorf("desktop executable directory is unavailable")
	}
	return directory, nil
}

func useDesktopPortableWorkingDirectory() error {
	if desktopBuild != "true" {
		return nil
	}
	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate desktop executable: %w", err)
	}
	directory, err := desktopRuntimeDirectory(executablePath)
	if err != nil {
		return err
	}
	if err := os.Chdir(directory); err != nil {
		return fmt.Errorf("switch to desktop directory: %w", err)
	}
	return nil
}

func normalizeSHA256(value string) (string, error) {
	digest := strings.TrimSpace(strings.ToLower(value))
	digest = strings.TrimPrefix(digest, "sha256:")
	if len(digest) != 64 {
		return "", fmt.Errorf("SHA-256 digest must contain 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return "", fmt.Errorf("invalid SHA-256 digest: %w", err)
	}
	return digest, nil
}

func findDesktopUpdateAsset(info latestReleaseInfo, arch string) (releaseAsset, bool) {
	wantedName := desktopUpdateAssetName(arch)
	for _, asset := range info.Assets {
		if !strings.EqualFold(asset.Name, wantedName) || asset.State != "uploaded" || asset.Size <= 0 || strings.TrimSpace(asset.BrowserDownloadURL) == "" {
			continue
		}
		if _, err := normalizeSHA256(asset.Digest); err == nil {
			return asset, true
		}
	}
	return releaseAsset{}, false
}

func currentDesktopUpdateAsset(info latestReleaseInfo) (releaseAsset, bool) {
	if !desktopSelfUpdateEnabled() {
		return releaseAsset{}, false
	}
	return findDesktopUpdateAsset(info, runtime.GOARCH)
}

func versionInfoPayload(info latestReleaseInfo) map[string]interface{} {
	hasUpdate := versionIsOlder(appVersion, info.TagName)
	payload := map[string]interface{}{
		"version":    appVersion,
		"latest":     info.TagName,
		"releaseURL": info.HTMLURL,
		"hasUpdate":  hasUpdate,
	}
	if hasUpdate {
		if asset, ok := currentDesktopUpdateAsset(info); ok {
			payload["oneClickUpdate"] = true
			payload["updateAssetName"] = asset.Name
			payload["updateSize"] = asset.Size
		}
	}
	return payload
}

func startOneClickDesktopUpdate(ctx context.Context, report func(string)) (string, error) {
	if !desktopSelfUpdateEnabled() {
		return "", fmt.Errorf("this build does not support one-click desktop updates")
	}
	desktopUpdateState.Lock()
	if desktopUpdateState.updating {
		desktopUpdateState.Unlock()
		return "", fmt.Errorf("an update is already in progress")
	}
	desktopUpdateState.updating = true
	desktopUpdateState.Unlock()
	updateStaged := false
	defer func() {
		if updateStaged {
			return
		}
		desktopUpdateState.Lock()
		desktopUpdateState.updating = false
		desktopUpdateState.Unlock()
	}()

	report("正在检查桌面更新包")
	info, err := getLatestRelease(ctx)
	if err != nil {
		return "", fmt.Errorf("check latest release: %w", err)
	}
	if !versionIsOlder(appVersion, info.TagName) {
		return "", fmt.Errorf("already running the latest version")
	}
	asset, ok := currentDesktopUpdateAsset(info)
	if !ok {
		return "", fmt.Errorf("release %s does not include a verified desktop update asset", info.TagName)
	}
	if err := stageDesktopUpdate(ctx, asset, report); err != nil {
		return "", err
	}
	updateStaged = true
	return info.TagName, nil
}

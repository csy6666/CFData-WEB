//go:build !windows

package main

import (
	"context"
	"fmt"
)

func runDesktopUpdateApplierIfRequested([]string) bool {
	return false
}

func stageDesktopUpdate(context.Context, releaseAsset, func(string)) error {
	return fmt.Errorf("desktop updates are only available on Windows")
}

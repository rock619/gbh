package gbh

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func chooseROMFile() (string, error) {
	const script = `POSIX path of (choose file with prompt "Open ROM")`
	cmd := exec.Command("osascript", "-e", script)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		if isAppleScriptCancel(stderr.String()) {
			return "", errFileDialogCancelled
		}
		return "", fmt.Errorf("osascript file dialog failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	filename := strings.TrimSpace(string(out))
	if filename == "" {
		return "", errFileDialogCancelled
	}
	return filename, nil
}

func isAppleScriptCancel(stderr string) bool {
	return strings.Contains(stderr, "User canceled") || strings.Contains(stderr, "ユーザによってキャンセル")
}

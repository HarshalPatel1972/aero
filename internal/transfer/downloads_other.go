//go:build !windows

package transfer

import (
	"os"
	"path/filepath"
)

func userDownloadsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Downloads"), nil
}

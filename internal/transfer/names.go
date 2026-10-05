// MIT License
// Copyright (c) 2026 Project AERO Contributors

package transfer

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

const maxNameBytes = 200

// windowsReserved are device names Windows refuses as file names, with or
// without an extension.
var windowsReserved = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// SanitizeFilename turns an untrusted name from the phone into a single,
// safe path component: no directories, no characters Windows forbids, no
// reserved device names, and a bounded length.
func SanitizeFilename(name string) string {
	// Keep only the last path element, whichever separator was used.
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}

	var b strings.Builder
	for _, r := range name {
		switch {
		case r == utf8.RuneError, unicode.IsControl(r), strings.ContainsRune(`<>:"|?*`, r):
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	name = strings.TrimRight(strings.TrimSpace(b.String()), ". ")

	if len(name) > maxNameBytes {
		ext := filepath.Ext(name)
		if len(ext) > 20 {
			ext = ""
		}
		stem := name[:maxNameBytes-len(ext)]
		for !utf8.ValidString(stem) {
			stem = stem[:len(stem)-1]
		}
		name = stem + ext
	}

	if name == "" || strings.Trim(name, ".") == "" {
		return "file"
	}
	stem := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	if windowsReserved[strings.TrimSpace(stem)] {
		name = "_" + name
	}
	return name
}

var renameMu sync.Mutex

// moveToUniqueName renames src into dir as name, appending " (1)", " (2)", …
// when a file with that name already exists. It never overwrites.
func moveToUniqueName(src, dir, name string) (string, error) {
	renameMu.Lock()
	defer renameMu.Unlock()

	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 0; i < 10000; i++ {
		candidate := name
		if i > 0 {
			candidate = fmt.Sprintf("%s (%d)%s", stem, i, ext)
		}
		dst := filepath.Join(dir, candidate)
		if _, err := os.Lstat(dst); errors.Is(err, fs.ErrNotExist) {
			return candidate, os.Rename(src, dst)
		}
	}
	return "", fmt.Errorf("no free file name for %q", name)
}

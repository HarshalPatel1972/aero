//go:build windows

package transfer

import "golang.org/x/sys/windows"

// userDownloadsDir asks Windows for the Downloads known folder, which honours
// users who have moved it (e.g. to another drive or OneDrive).
func userDownloadsDir() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_Downloads, 0)
}

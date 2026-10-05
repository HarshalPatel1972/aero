//go:build windows

package transfer

import "golang.org/x/sys/windows"

// userDownloadsDir asks Windows for the Downloads known folder, which honours
// users who have moved it (e.g. to another drive or OneDrive).
func userDownloadsDir() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_Downloads, 0)
}

// hidePath sets the Windows hidden attribute so staging data stays out of sight.
func hidePath(path string) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	if attrs, err := windows.GetFileAttributes(p); err == nil {
		windows.SetFileAttributes(p, attrs|windows.FILE_ATTRIBUTE_HIDDEN)
	}
}

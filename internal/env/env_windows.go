//go:build windows

package env

// LoadShellEnvironment returns an empty map on Windows.
// Windows applications launched from Explorer typically inherit
// the system environment variables correctly, so no special
// handling is needed.
func LoadShellEnvironment() map[string]string {
	return make(map[string]string)
}

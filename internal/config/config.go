package config

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// CopyToClipboardName is the name of the special "Copy to Clipboard" terminal option.
const CopyToClipboardName = "Copy to Clipboard"

// Terminal represents a detected terminal emulator.
type Terminal struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// Settings represents the application settings.
type Settings struct {
	Terminals        []Terminal `json:"terminals"`
	SelectedTerminal string     `json:"selected_terminal"`
}

// configDir returns the path to the config directory.
func configDir() (string, error) {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(userConfigDir, "harbor"), nil
}

// configPath returns the path to the config file.
func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads the settings from the config file.
// If the file doesn't exist, it creates default settings.
func Load() (*Settings, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Create default settings
			settings := &Settings{}
			detectedTerminals := DetectTerminals()
			// Always include the clipboard option
			clipboardTerminal := Terminal{Name: CopyToClipboardName, Path: ""}
			settings.Terminals = append([]Terminal{clipboardTerminal}, detectedTerminals...)
			// If no terminals detected, default to clipboard; otherwise use first detected terminal
			if len(detectedTerminals) == 0 {
				settings.SelectedTerminal = CopyToClipboardName
			} else {
				settings.SelectedTerminal = detectedTerminals[0].Name
			}
			// Save the default settings
			if saveErr := settings.Save(); saveErr != nil {
				return nil, saveErr
			}
			return settings, nil
		}
		return nil, err
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	// Ensure clipboard option is always present
	hasClipboard := false
	for _, t := range settings.Terminals {
		if t.Name == CopyToClipboardName {
			hasClipboard = true
			break
		}
	}
	if !hasClipboard {
		clipboardTerminal := Terminal{Name: CopyToClipboardName, Path: ""}
		settings.Terminals = append([]Terminal{clipboardTerminal}, settings.Terminals...)
	}

	return &settings, nil
}

// Save writes the settings to the config file.
func (s *Settings) Save() error {
	dir, err := configDir()
	if err != nil {
		return err
	}

	// Ensure the config directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GetSelectedTerminal returns the currently selected terminal.
// Returns nil if no terminal is selected or found.
func (s *Settings) GetSelectedTerminal() *Terminal {
	for i := range s.Terminals {
		if s.Terminals[i].Name == s.SelectedTerminal {
			return &s.Terminals[i]
		}
	}
	return nil
}

// IsCopyToClipboard returns true if this terminal is the "Copy to Clipboard" option.
func (t *Terminal) IsCopyToClipboard() bool {
	return t.Name == CopyToClipboardName
}

// DetectTerminals finds installed terminal emulators on the system.
func DetectTerminals() []Terminal {
	var terminals []Terminal

	switch runtime.GOOS {
	case "darwin":
		terminals = detectDarwinTerminals()
	case "linux":
		terminals = detectLinuxTerminals()
	case "windows":
		terminals = detectWindowsTerminals()
	}

	return terminals
}

// detectDarwinTerminals detects terminals on macOS.
func detectDarwinTerminals() []Terminal {
	var terminals []Terminal

	// Check for common terminal emulators
	darwinTerminals := []struct {
		name    string
		binName string
	}{
		{"ghostty", "ghostty"},
		{"kitty", "kitty"},
		{"wezterm", "wezterm"},
		{"alacritty", "alacritty"},
	}

	for _, t := range darwinTerminals {
		if path, err := exec.LookPath(t.binName); err == nil {
			terminals = append(terminals, Terminal{Name: t.name, Path: path})
		}
	}

	// Check for Terminal.app (always available on macOS)
	terminalAppPath := "/System/Applications/Utilities/Terminal.app"
	if _, err := os.Stat(terminalAppPath); err == nil {
		terminals = append(terminals, Terminal{Name: "Terminal.app", Path: terminalAppPath})
	}

	return terminals
}

// detectLinuxTerminals detects terminals on Linux.
func detectLinuxTerminals() []Terminal {
	var terminals []Terminal

	linuxTerminals := []struct {
		name    string
		binName string
	}{
		{"ghostty", "ghostty"},
		{"kitty", "kitty"},
		{"wezterm", "wezterm"},
		{"alacritty", "alacritty"},
		{"gnome-terminal", "gnome-terminal"},
		{"konsole", "konsole"},
		{"xfce4-terminal", "xfce4-terminal"},
	}

	for _, t := range linuxTerminals {
		if path, err := exec.LookPath(t.binName); err == nil {
			terminals = append(terminals, Terminal{Name: t.name, Path: path})
		}
	}

	return terminals
}

// detectWindowsTerminals detects terminals on Windows.
func detectWindowsTerminals() []Terminal {
	var terminals []Terminal

	// Check for Windows Terminal
	if path, err := exec.LookPath("wt.exe"); err == nil {
		terminals = append(terminals, Terminal{Name: "wt", Path: path})
	}

	// CMD is always available on Windows
	if path, err := exec.LookPath("cmd.exe"); err == nil {
		terminals = append(terminals, Terminal{Name: "cmd", Path: path})
	}

	return terminals
}

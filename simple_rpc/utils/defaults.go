package utils

import (
	"github.com/ethereum/go-ethereum/cmd/utils"
	"os"
	"path/filepath"
	"runtime"
)

// DefaultConfigDir is the default config directory to use for the vaults and other
// persistence requirements.
func DefaultConfigDir() string {
	// Try to place the data folder in the user's home dir
	home := utils.HomeDir()
	if home != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "Orchestrator")
		} else if runtime.GOOS == "windows" {
			appdata := os.Getenv("APPDATA")
			if appdata != "" {
				return filepath.Join(appdata, "Orchestrator")
			} else {
				return filepath.Join(home, "AppData", "Roaming", "Orchestrator")
			}
		} else {
			return filepath.Join(home, ".Orchestrator")
		}
	}
	// As we cannot guess a stable location, return empty and handle later
	return ""
}

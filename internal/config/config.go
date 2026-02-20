// Package config manages tailchat user configuration (theme selection, etc.)
// stored at ~/.tailchat/config.json.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds user preferences.
type Config struct {
	ActiveTheme string `json:"activeTheme"` // "default" or a theme directory name
}

// ThemeInfo describes an installed theme.
type ThemeInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Path        string `json:"path"` // filesystem path
	IsDefault   bool   `json:"isDefault"`
}

// ThemeMeta is the optional theme.json in a theme directory.
type ThemeMeta struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Version     string `json:"version"`
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".tailchat")
}

func configPath() string {
	return filepath.Join(configDir(), "config.json")
}

// ThemesDir returns the path to ~/.tailchat/themes/.
func ThemesDir() string {
	return filepath.Join(configDir(), "themes")
}

// Load reads the config from disk. Returns defaults if not found.
func Load() *Config {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return &Config{ActiveTheme: "default"}
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return &Config{ActiveTheme: "default"}
	}
	if cfg.ActiveTheme == "" {
		cfg.ActiveTheme = "default"
	}
	return &cfg
}

// Save writes the config to disk.
func Save(cfg *Config) error {
	if err := os.MkdirAll(configDir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0o644)
}

// ListThemes returns all installed themes (including "default").
func ListThemes() []ThemeInfo {
	themes := []ThemeInfo{
		{
			Name:        "default",
			Description: "Built-in default theme",
			Author:      "tailchat",
			IsDefault:   true,
		},
	}

	themesDir := ThemesDir()
	entries, err := os.ReadDir(themesDir)
	if err != nil {
		return themes
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		themeDir := filepath.Join(themesDir, entry.Name())
		indexPath := filepath.Join(themeDir, "index.html")
		if _, err := os.Stat(indexPath); err != nil {
			continue // no index.html, skip
		}

		info := ThemeInfo{
			Name: entry.Name(),
			Path: themeDir,
		}

		// Try to load theme.json for metadata
		metaPath := filepath.Join(themeDir, "theme.json")
		if data, err := os.ReadFile(metaPath); err == nil {
			var meta ThemeMeta
			if json.Unmarshal(data, &meta) == nil {
				if meta.Name != "" {
					info.Name = meta.Name
				}
				info.Description = meta.Description
				info.Author = meta.Author
			}
		}

		themes = append(themes, info)
	}

	return themes
}

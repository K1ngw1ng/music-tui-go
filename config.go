package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	MusicDir  string            `json:"music_dir"`
	Playlists map[string]string `json:"playlists,omitempty"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "musicplayer", "config.json")
}

func loadConfig() Config {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return Config{}
	}
	var c Config
	json.Unmarshal(data, &c)
	if c.Playlists == nil {
		c.Playlists = map[string]string{}
	}
	return c
}

func saveConfig(c Config) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func resolvePlaylist(cfg Config, name string) (string, error) {
	path, ok := cfg.Playlists[name]
	if !ok {
		return "", fmt.Errorf("unknown playlist %q (use --create-playlist to create it)", name)
	}
	return path, nil
}

func registerPlaylist(cfg *Config, name, path string) error {
	cfg.Playlists[name] = path
	return saveConfig(*cfg)
}
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	cfg := loadConfig()

	dir := ""
	if len(os.Args) > 1 {
		dir = os.Args[1]
	} else if cfg.MusicDir != "" {
		dir = cfg.MusicDir
	} else {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}

	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "error: %q is not a directory\n", dir)
		os.Exit(1)
	}

	if dir != cfg.MusicDir {
		cfg.MusicDir = dir
		saveConfig(cfg)
	}

	fmt.Printf("Scanning %s...\n", dir)
	tracks, err := scanDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error scanning: %v\n", err)
		os.Exit(1)
	}
	if len(tracks) == 0 {
		fmt.Fprintf(os.Stderr, "No audio files found in %s\n", dir)
		os.Exit(1)
	}

	m := newModel(tracks)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
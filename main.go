package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Flags
	albumFlag := flag.String("album", "", "Show only tracks from this album")
	artistFlag := flag.String("artist", "", "Show only tracks from this artist")
	playlistFlag := flag.String("playlist", "", "Open a named playlist")
	createFlag := flag.String("create-playlist", "", "Create (or overwrite) a named playlist from paths")
	addFlag := flag.String("add-to-playlist", "", "Add tracks to a named playlist from paths")
	outputFlag := flag.String("output", "", "Output .m3u path for --create-playlist (default: MusicDir/<name>.m3u)")
	lastfmAuthFlag := flag.Bool("lastfm-auth", false, "Set up Last.fm API credentials and authenticate")
	flag.Parse()

	cfg := loadConfig()
	if cfg.Playlists == nil {
		cfg.Playlists = map[string]string{}
	}

	if *lastfmAuthFlag {
		if err := lastfmAuthFlow(); err != nil {
			fmt.Fprintf(os.Stderr, "last.fm auth error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *createFlag != "" {
		m3uPath := *outputFlag
		if m3uPath == "" {
			if cfg.MusicDir == "" {
				fmt.Fprintln(os.Stderr, "error: no --output given and no music_dir configured")
				os.Exit(1)
			}
			m3uPath = filepath.Join(cfg.MusicDir, *createFlag+".m3u")
		}

		if err := os.WriteFile(m3uPath, []byte("#EXTM3U\n"), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error creating playlist file: %v\n", err)
			os.Exit(1)
		}

		if paths := flag.Args(); len(paths) > 0 {
			tracks, err := scanPaths(paths)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error scanning: %v\n", err)
				os.Exit(1)
			}
			if err := appendM3U(m3uPath, tracks); err != nil {
				fmt.Fprintf(os.Stderr, "error writing playlist: %v\n", err)
				os.Exit(1)
			}
		}

		if err := registerPlaylist(&cfg, *createFlag, m3uPath); err != nil {
			fmt.Fprintf(os.Stderr, "error saving config: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Created playlist %q at %s\n", *createFlag, m3uPath)
		return
	}

	if *addFlag != "" {
		m3uPath, err := resolvePlaylist(cfg, *addFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}

		paths := flag.Args()
		if len(paths) == 0 {
			if cfg.MusicDir == "" {
				fmt.Fprintln(os.Stderr, "error: no paths given and no music_dir configured")
				os.Exit(1)
			}
			paths = []string{cfg.MusicDir}
		}

		tracks, err := scanPaths(paths)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error scanning: %v\n", err)
			os.Exit(1)
		}
		if err := appendM3U(m3uPath, tracks); err != nil {
			fmt.Fprintf(os.Stderr, "error appending to playlist: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Added %d tracks to playlist %q\n", len(tracks), *addFlag)
		return
	}

	if *playlistFlag != "" {
		m3uPath, err := resolvePlaylist(cfg, *playlistFlag)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		tracks, err := tracksFromM3U(m3uPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading playlist: %v\n", err)
			os.Exit(1)
		}
		if len(tracks) == 0 {
			fmt.Fprintf(os.Stderr, "Playlist %q is empty\n", *playlistFlag)
			os.Exit(1)
		}
		runTUI(tracks, cfg, *playlistFlag, loadLastFMClient())
		return
	}

	dir := ""
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
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

	title := "Library"
	if *albumFlag != "" {
		tracks = filterByAlbum(tracks, *albumFlag)
		title = "Album: " + *albumFlag
		if len(tracks) == 0 {
			fmt.Fprintf(os.Stderr, "No tracks found for album %q\n", *albumFlag)
			os.Exit(1)
		}
	} else if *artistFlag != "" {
		tracks = filterByArtist(tracks, *artistFlag)
		title = "Artist: " + *artistFlag
		if len(tracks) == 0 {
			fmt.Fprintf(os.Stderr, "No tracks found for artist %q\n", *artistFlag)
			os.Exit(1)
		}
	}

	runTUI(tracks, cfg, title, loadLastFMClient())
}

func runTUI(tracks []Track, cfg Config, title string, lastfm *LastFMClient) {
	m := newModel(tracks, cfg, title, lastfm)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
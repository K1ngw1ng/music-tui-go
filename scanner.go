package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var audioExts = map[string]bool{
	".mp3": true, ".flac": true, ".ogg": true,
	".m4a": true, ".aac": true, ".wav": true,
	".opus": true, ".wma": true,
}

type Track struct {
	Path   string
	Title  string
	Artist string
	Album  string
}

func (t Track) DisplayTitle() string {
	if t.Title != "" {
		return t.Title
	}
	return strings.TrimSuffix(filepath.Base(t.Path), filepath.Ext(t.Path))
}

func (t Track) DisplayArtist() string {
	if t.Artist != "" {
		return t.Artist
	}
	return "Unknown Artist"
}

func scanDir(dir string) ([]Track, error) {
	var tracks []Track
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if audioExts[strings.ToLower(filepath.Ext(path))] {
			tracks = append(tracks, probeTrack(path))
		}
		return nil
	})
	return tracks, err
}

type ffprobeOutput struct {
	Format struct {
		Tags map[string]string `json:"tags"`
	} `json:"format"`
}

func probeTrack(path string) Track {
	t := Track{Path: path}
	out, err := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		path,
	).Output()
	if err != nil {
		return t
	}
	var probe ffprobeOutput
	if err := json.Unmarshal(out, &probe); err != nil {
		return t
	}
	tags := probe.Format.Tags
	t.Title = firstNonEmpty(tags["title"], tags["TITLE"])
	t.Artist = firstNonEmpty(tags["artist"], tags["ARTIST"], tags["album_artist"], tags["ALBUM_ARTIST"])
	t.Album = firstNonEmpty(tags["album"], tags["ALBUM"])
	return t
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

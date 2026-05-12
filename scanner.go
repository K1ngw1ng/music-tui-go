package main

import (
	"bufio"
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

func scanPaths(paths []string) ([]Track, error) {
	var tracks []Track
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			ts, err := scanDir(p)
			if err != nil {
				return nil, err
			}
			tracks = append(tracks, ts...)
		} else if audioExts[strings.ToLower(filepath.Ext(p))] {
			tracks = append(tracks, probeTrack(p))
		}
	}
	return tracks, nil
}

func filterByAlbum(tracks []Track, album string) []Track {
	album = strings.ToLower(album)
	var out []Track
	for _, t := range tracks {
		if strings.ToLower(t.Album) == album {
			out = append(out, t)
		}
	}
	return out
}

func filterByArtist(tracks []Track, artist string) []Track {
	artist = strings.ToLower(artist)
	var out []Track
	for _, t := range tracks {
		if strings.ToLower(t.Artist) == artist {
			out = append(out, t)
		}
	}
	return out
}

func readM3U(m3uPath string) ([]string, error) {
	f, err := os.Open(m3uPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dir := filepath.Dir(m3uPath)
	var paths []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !filepath.IsAbs(line) {
			line = filepath.Join(dir, line)
		}
		paths = append(paths, line)
	}
	return paths, sc.Err()
}

func appendM3U(m3uPath string, tracks []Track) error {
	existing := map[string]bool{}
	if paths, err := readM3U(m3uPath); err == nil {
		for _, p := range paths {
			existing[p] = true
		}
	}

	f, err := os.OpenFile(m3uPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	info, _ := f.Stat()
	if info.Size() == 0 {
		f.WriteString("#EXTM3U\n")
	}

	for _, t := range tracks {
		if !existing[t.Path] {
			f.WriteString(t.Path + "\n")
		}
	}
	return nil
}

func tracksFromM3U(m3uPath string) ([]Track, error) {
	paths, err := readM3U(m3uPath)
	if err != nil {
		return nil, err
	}
	var tracks []Track
	for _, p := range paths {
		if audioExts[strings.ToLower(filepath.Ext(p))] {
			tracks = append(tracks, probeTrack(p))
		}
	}
	return tracks, nil
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
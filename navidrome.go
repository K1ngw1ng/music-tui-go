package main

import (
	"bufio"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	subsonicVersion = "1.16.1"
	subsonicClient  = "musicplayer"
)

type NavidromeConfig struct {
	Server   string `json:"server"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type NavidromeClient struct {
	cfg   NavidromeConfig
	salt  string
	token string
}

func navidromeConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "musicplayer", "navidrome.json")
}

func loadNavidromeConfig() (NavidromeConfig, error) {
	data, err := os.ReadFile(navidromeConfigPath())
	if err != nil {
		return NavidromeConfig{}, err
	}
	var cfg NavidromeConfig
	err = json.Unmarshal(data, &cfg)
	return cfg, err
}

func saveNavidromeConfig(cfg NavidromeConfig) error {
	path := navidromeConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func newNavidromeClient(cfg NavidromeConfig) *NavidromeClient {
	salt := randomSalt()
	h := md5.Sum([]byte(cfg.Password + salt))
	token := fmt.Sprintf("%x", h)
	return &NavidromeClient{cfg: cfg, salt: salt, token: token}
}

func loadNavidromeClient() *NavidromeClient {
	cfg, err := loadNavidromeConfig()
	if err != nil || cfg.Server == "" {
		return nil
	}
	return newNavidromeClient(cfg)
}

func randomSalt() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[r.Intn(len(chars))]
	}
	return string(b)
}

func (c *NavidromeClient) authValues() url.Values {
	v := url.Values{}
	v.Set("u", c.cfg.Username)
	v.Set("t", c.token)
	v.Set("s", c.salt)
	v.Set("v", subsonicVersion)
	v.Set("c", subsonicClient)
	v.Set("f", "json")
	return v
}

func (c *NavidromeClient) apiURL(method string, extra url.Values) string {
	params := c.authValues()
	for k, vals := range extra {
		params[k] = vals
	}
	base := strings.TrimRight(c.cfg.Server, "/")
	return base + "/rest/" + method + "?" + params.Encode()
}

func (c *NavidromeClient) streamURL(songID string) string {
	return c.apiURL("stream", url.Values{"id": {songID}})
}

func (c *NavidromeClient) coverArtURL(coverArtID string) string {
	return c.apiURL("getCoverArt", url.Values{"id": {coverArtID}, "size": {"500"}})
}

func (c *NavidromeClient) apiGet(method string, extra url.Values) ([]byte, error) {
	resp, err := http.Get(c.apiURL(method, extra))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func checkSubsonicStatus(body []byte) error {
	var r struct {
		Response struct {
			Status string `json:"status"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		} `json:"subsonic-response"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return err
	}
	if r.Response.Status != "ok" {
		if r.Response.Error != nil {
			return fmt.Errorf("subsonic error %d: %s", r.Response.Error.Code, r.Response.Error.Message)
		}
		return fmt.Errorf("subsonic status: %s", r.Response.Status)
	}
	return nil
}

type subsonicAlbumSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type subsonicSong struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Album    string `json:"album"`
	Duration int    `json:"duration"`
	CoverArt string `json:"coverArt"`
}

func (c *NavidromeClient) fetchAlbumPage(offset int) ([]subsonicAlbumSummary, error) {
	extra := url.Values{
		"type":   {"alphabeticalByName"},
		"size":   {"500"},
		"offset": {fmt.Sprintf("%d", offset)},
	}
	body, err := c.apiGet("getAlbumList2", extra)
	if err != nil {
		return nil, err
	}
	if err := checkSubsonicStatus(body); err != nil {
		return nil, err
	}
	var r struct {
		Response struct {
			AlbumList2 struct {
				Album []subsonicAlbumSummary `json:"album"`
			} `json:"albumList2"`
		} `json:"subsonic-response"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, err
	}
	return r.Response.AlbumList2.Album, nil
}

func (c *NavidromeClient) fetchAlbumSongs(albumID string) (songs []subsonicSong, coverArt string, err error) {
	body, err := c.apiGet("getAlbum", url.Values{"id": {albumID}})
	if err != nil {
		return nil, "", err
	}
	if err := checkSubsonicStatus(body); err != nil {
		return nil, "", err
	}
	var r struct {
		Response struct {
			Album struct {
				CoverArt string         `json:"coverArt"`
				Song     []subsonicSong `json:"song"`
			} `json:"album"`
		} `json:"subsonic-response"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, "", err
	}
	return r.Response.Album.Song, r.Response.Album.CoverArt, nil
}

// FetchAllTracks pages through all albums and returns a flat track list.
func (c *NavidromeClient) FetchAllTracks() ([]Track, error) {
	var tracks []Track
	for offset := 0; ; offset += 500 {
		albums, err := c.fetchAlbumPage(offset)
		if err != nil {
			return nil, fmt.Errorf("fetching album list: %w", err)
		}
		for _, album := range albums {
			songs, albumCoverArt, err := c.fetchAlbumSongs(album.ID)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: skipping album %q: %v\n", album.Name, err)
				continue
			}
			for _, song := range songs {
				coverArt := song.CoverArt
				if coverArt == "" {
					coverArt = albumCoverArt
				}
				artURL := ""
				if coverArt != "" {
					artURL = c.coverArtURL(coverArt)
				}
				tracks = append(tracks, Track{
					Path:     c.streamURL(song.ID),
					Title:    song.Title,
					Artist:   song.Artist,
					Album:    song.Album,
					Duration: song.Duration,
					ArtURL:   artURL,
				})
			}
		}
		if len(albums) < 500 {
			break
		}
	}
	return tracks, nil
}

func navidromeAuthFlow() error {
	sc := bufio.NewScanner(os.Stdin)

	fmt.Print("Navidrome server URL (e.g. http://localhost:4533): ")
	sc.Scan()
	server := strings.TrimSpace(sc.Text())

	fmt.Print("Username: ")
	sc.Scan()
	username := strings.TrimSpace(sc.Text())

	fmt.Print("Password: ")
	sc.Scan()
	password := strings.TrimSpace(sc.Text())

	if server == "" || username == "" || password == "" {
		return fmt.Errorf("all fields are required")
	}

	cfg := NavidromeConfig{Server: server, Username: username, Password: password}
	client := newNavidromeClient(cfg)

	body, err := client.apiGet("ping", nil)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	if err := checkSubsonicStatus(body); err != nil {
		return fmt.Errorf("auth failed: %w", err)
	}

	if err := saveNavidromeConfig(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}
	fmt.Println("Navidrome credentials saved.")
	return nil
}

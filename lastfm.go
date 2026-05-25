package main

import (
	"bufio"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const lastfmAPI = "https://ws.audioscrobbler.com/2.0/"

type LastFMConfig struct {
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
	SessionKey string `json:"session_key"`
}

type LastFMClient struct {
	cfg LastFMConfig
}

func lastfmConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "musicplayer", "lastfm.json")
}

func loadLastFMConfig() (LastFMConfig, error) {
	data, err := os.ReadFile(lastfmConfigPath())
	if err != nil {
		return LastFMConfig{}, err
	}
	var cfg LastFMConfig
	err = json.Unmarshal(data, &cfg)
	return cfg, err
}

func saveLastFMConfig(cfg LastFMConfig) error {
	path := lastfmConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func loadLastFMClient() *LastFMClient {
	cfg, err := loadLastFMConfig()
	if err != nil || cfg.APIKey == "" || cfg.APISecret == "" || cfg.SessionKey == "" {
		return nil
	}
	return &LastFMClient{cfg: cfg}
}

func (c *LastFMClient) sign(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString(params[k])
	}
	sb.WriteString(c.cfg.APISecret)

	h := md5.Sum([]byte(sb.String()))
	return fmt.Sprintf("%x", h)
}

func (c *LastFMClient) post(params map[string]string) error {
	params["api_sig"] = c.sign(params)
	params["format"] = "json"

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}

	resp, err := http.PostForm(lastfmAPI, form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}
	if errObj, ok := result["error"]; ok {
		msg, _ := result["message"].(string)
		return fmt.Errorf("last.fm error %v: %s", errObj, msg)
	}
	return nil
}

func (c *LastFMClient) NowPlaying(track Track) {
	if c == nil {
		return
	}
	go func() {
		params := map[string]string{
			"method":  "track.updateNowPlaying",
			"api_key": c.cfg.APIKey,
			"sk":      c.cfg.SessionKey,
			"artist":  track.DisplayArtist(),
			"track":   track.DisplayTitle(),
		}
		if track.Album != "" {
			params["album"] = track.Album
		}
		if track.Duration > 0 {
			params["duration"] = fmt.Sprintf("%d", track.Duration)
		}
		if err := c.post(params); err != nil {
			fmt.Fprintf(os.Stderr, "last.fm now playing: %v\n", err)
		}
	}()
}

func (c *LastFMClient) Scrobble(track Track, startedAt time.Time) {
	if c == nil {
		return
	}
	go func() {
		params := map[string]string{
			"method":    "track.scrobble",
			"api_key":   c.cfg.APIKey,
			"sk":        c.cfg.SessionKey,
			"artist":    track.DisplayArtist(),
			"track":     track.DisplayTitle(),
			"timestamp": fmt.Sprintf("%d", startedAt.Unix()),
		}
		if track.Album != "" {
			params["album"] = track.Album
		}
		if track.Duration > 0 {
			params["duration"] = fmt.Sprintf("%d", track.Duration)
		}
		if err := c.post(params); err != nil {
			fmt.Fprintf(os.Stderr, "last.fm scrobble: %v\n", err)
		}
	}()
}

func startScrobbleTimer(track Track, client *LastFMClient) func() {
	if client == nil {
		return func() {}
	}

	client.NowPlaying(track)

	threshold := track.Duration / 2
	if threshold > 240 {
		threshold = 240
	}
	if threshold < 1 {
		threshold = 240
	}

	startedAt := time.Now()
	done := make(chan struct{})

	go func() {
		select {
		case <-time.After(time.Duration(threshold) * time.Second):
			client.Scrobble(track, startedAt)
		case <-done:
		}
	}()

	return func() {
		select {
		case <-done:
		default:
			close(done)
		}
	}
}

func lastfmAuthFlow() error {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Print("Last.fm API key: ")
	scanner.Scan()
	apiKey := strings.TrimSpace(scanner.Text())

	fmt.Print("Last.fm API secret: ")
	scanner.Scan()
	apiSecret := strings.TrimSpace(scanner.Text())

	if apiKey == "" || apiSecret == "" {
		return fmt.Errorf("API key and secret are required")
	}

	resp, err := http.Get(lastfmAPI + "?method=auth.getToken&api_key=" + apiKey + "&format=json")
	if err != nil {
		return fmt.Errorf("getting token: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var tokenResp struct {
		Token string `json:"token"`
		Error int    `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("parsing token response: %w", err)
	}
	if tokenResp.Error != 0 {
		return fmt.Errorf("last.fm error %d: %s", tokenResp.Error, tokenResp.Message)
	}
	token := tokenResp.Token

	authURL := fmt.Sprintf("https://www.last.fm/api/auth/?api_key=%s&token=%s", apiKey, token)
	fmt.Printf("\nOpen this URL in your browser and approve access:\n\n  %s\n\nPress Enter when done...", authURL)
	scanner.Scan()

	sigInput := fmt.Sprintf("api_key%smethodauth.getSessiontoken%s%s", apiKey, token, apiSecret)
	h := md5.Sum([]byte(sigInput))
	sig := fmt.Sprintf("%x", h)

	sessionURL := fmt.Sprintf("%s?method=auth.getSession&api_key=%s&token=%s&api_sig=%s&format=json",
		lastfmAPI, apiKey, token, sig)
	resp2, err := http.Get(sessionURL)
	if err != nil {
		return fmt.Errorf("getting session: %w", err)
	}
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(resp2.Body)

	var sessionResp struct {
		Session struct {
			Key string `json:"key"`
		} `json:"session"`
		Error   int    `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body2, &sessionResp); err != nil {
		return fmt.Errorf("parsing session response: %w", err)
	}
	if sessionResp.Error != 0 {
		return fmt.Errorf("last.fm error %d: %s", sessionResp.Error, sessionResp.Message)
	}

	cfg := LastFMConfig{
		APIKey:     apiKey,
		APISecret:  apiSecret,
		SessionKey: sessionResp.Session.Key,
	}
	if err := saveLastFMConfig(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Println("\nLast.fm authentication successful! Credentials saved.")
	return nil
}
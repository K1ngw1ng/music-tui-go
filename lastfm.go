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

type scrobbleService struct {
	name    string
	api     string
	authURL string
	file    string
}

var (
	lastfmService  = scrobbleService{"Last.fm", "https://ws.audioscrobbler.com/2.0/", "https://www.last.fm/api/auth/", "lastfm.json"}
	librefmService = scrobbleService{"Libre.fm", "https://libre.fm/2.0/", "https://libre.fm/api/auth/", "librefm.json"}
)

type LastFMConfig struct {
	APIKey     string `json:"api_key"`
	APISecret  string `json:"api_secret"`
	SessionKey string `json:"session_key"`
}

type scrobbleTarget struct {
	svc scrobbleService
	cfg LastFMConfig
}

// LastFMClient scrobbles to every configured service (Last.fm, Libre.fm).
type LastFMClient struct {
	targets []scrobbleTarget
}

func lastfmConfigPath(svc scrobbleService) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "musicplayer", svc.file)
}

func loadLastFMConfig(svc scrobbleService) (LastFMConfig, error) {
	data, err := os.ReadFile(lastfmConfigPath(svc))
	if err != nil {
		return LastFMConfig{}, err
	}
	var cfg LastFMConfig
	err = json.Unmarshal(data, &cfg)
	return cfg, err
}

func saveLastFMConfig(svc scrobbleService, cfg LastFMConfig) error {
	path := lastfmConfigPath(svc)
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
	var targets []scrobbleTarget
	for _, svc := range []scrobbleService{lastfmService, librefmService} {
		cfg, err := loadLastFMConfig(svc)
		if err != nil || cfg.APIKey == "" || cfg.APISecret == "" || cfg.SessionKey == "" {
			continue
		}
		targets = append(targets, scrobbleTarget{svc, cfg})
	}
	if len(targets) == 0 {
		return nil
	}
	return &LastFMClient{targets: targets}
}

func (t scrobbleTarget) sign(params map[string]string) string {
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
	sb.WriteString(t.cfg.APISecret)

	h := md5.Sum([]byte(sb.String()))
	return fmt.Sprintf("%x", h)
}

func (t scrobbleTarget) post(params map[string]string) error {
	params["api_sig"] = t.sign(params)
	params["format"] = "json"

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}

	resp, err := http.PostForm(t.svc.api, form)
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
		return fmt.Errorf("%s error %v: %s", t.svc.name, errObj, msg)
	}
	return nil
}

func (c *LastFMClient) NowPlaying(track Track) {
	if c == nil {
		return
	}
	for _, t := range c.targets {
		go func(t scrobbleTarget) {
			params := map[string]string{
				"method":  "track.updateNowPlaying",
				"api_key": t.cfg.APIKey,
				"sk":      t.cfg.SessionKey,
				"artist":  track.DisplayArtist(),
				"track":   track.DisplayTitle(),
			}
			if track.Album != "" {
				params["album"] = track.Album
			}
			if track.Duration > 0 {
				params["duration"] = fmt.Sprintf("%d", track.Duration)
			}
			if err := t.post(params); err != nil {
				fmt.Fprintf(os.Stderr, "%s now playing: %v\n", t.svc.name, err)
			}
		}(t)
	}
}

func (c *LastFMClient) Scrobble(track Track, startedAt time.Time) {
	if c == nil {
		return
	}
	for _, t := range c.targets {
		go func(t scrobbleTarget) {
			params := map[string]string{
				"method":    "track.scrobble",
				"api_key":   t.cfg.APIKey,
				"sk":        t.cfg.SessionKey,
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
			if err := t.post(params); err != nil {
				fmt.Fprintf(os.Stderr, "%s scrobble: %v\n", t.svc.name, err)
			}
		}(t)
	}
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

func lastfmAuthFlow(svc scrobbleService) error {
	scanner := bufio.NewScanner(os.Stdin)

	apiKey, apiSecret := "musicplayer", "musicplayer"
	if svc != librefmService { // Libre.fm accepts any key and secret
		fmt.Printf("%s API key: ", svc.name)
		scanner.Scan()
		apiKey = strings.TrimSpace(scanner.Text())

		fmt.Printf("%s API secret: ", svc.name)
		scanner.Scan()
		apiSecret = strings.TrimSpace(scanner.Text())

		if apiKey == "" || apiSecret == "" {
			return fmt.Errorf("API key and secret are required")
		}
	}

	resp, err := http.Get(svc.api + "?method=auth.getToken&api_key=" + apiKey + "&format=json")
	if err != nil {
		return fmt.Errorf("getting token: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var tokenResp struct {
		Token   string `json:"token"`
		Error   int    `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("parsing token response: %w", err)
	}
	if tokenResp.Error != 0 {
		return fmt.Errorf("%s error %d: %s", svc.name, tokenResp.Error, tokenResp.Message)
	}
	token := tokenResp.Token

	authURL := fmt.Sprintf("%s?api_key=%s&token=%s", svc.authURL, apiKey, token)
	fmt.Printf("\nOpen this URL in your browser and approve access:\n\n  %s\n\nPress Enter when done...", authURL)
	scanner.Scan()

	sigInput := fmt.Sprintf("api_key%smethodauth.getSessiontoken%s%s", apiKey, token, apiSecret)
	h := md5.Sum([]byte(sigInput))
	sig := fmt.Sprintf("%x", h)

	sessionURL := fmt.Sprintf("%s?method=auth.getSession&api_key=%s&token=%s&api_sig=%s&format=json",
		svc.api, apiKey, token, sig)
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
		return fmt.Errorf("%s error %d: %s", svc.name, sessionResp.Error, sessionResp.Message)
	}

	cfg := LastFMConfig{
		APIKey:     apiKey,
		APISecret:  apiSecret,
		SessionKey: sessionResp.Session.Key,
	}
	if err := saveLastFMConfig(svc, cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Printf("\n%s authentication successful! Credentials saved.\n", svc.name)
	return nil
}

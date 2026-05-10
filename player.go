package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

var audioBackend string

func init() {
	audioBackend = detectBackend()
}

func detectBackend() string {
	for _, tool := range []string{"paplay", "aplay"} {
		if p, err := exec.LookPath(tool); err == nil && p != "" {
			return tool
		}
	}
	return ""
}

type Player struct {
	mu      sync.Mutex
	ffmpeg  *exec.Cmd
	sink    *exec.Cmd
	paused  bool
	stopped bool
}

func (p *Player) Play(path string, onDone func()) {
	p.mu.Lock()
	p.stop()
	p.stopped = false
	p.paused = false

	if audioBackend == "" {
		p.mu.Unlock()
		fmt.Fprintf(os.Stderr, "no audio backend found (install pulseaudio-utils or alsa-utils)\n")
		return
	}

	rate, channels := probeAudio(path)

	ffmpegCmd := exec.Command("ffmpeg",
		"-hide_banner", "-loglevel", "quiet",
		"-i", path,
		"-vn",
		"-f", "s16le",
		"-ar", fmt.Sprintf("%d", rate),
		"-ac", fmt.Sprintf("%d", channels),
		"pipe:1",
	)

	var sinkCmd *exec.Cmd
	switch audioBackend {
	case "paplay":
		sinkCmd = exec.Command("paplay",
			"--raw",
			fmt.Sprintf("--rate=%d", rate),
			fmt.Sprintf("--channels=%d", channels),
			"--format=s16le",
		)
	case "aplay":
		sinkCmd = exec.Command("aplay",
			"-q",
			"-f", "S16_LE",
			"-r", fmt.Sprintf("%d", rate),
			"-c", fmt.Sprintf("%d", channels),
		)
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		p.mu.Unlock()
		fmt.Fprintf(os.Stderr, "pipe error: %v\n", err)
		return
	}

	ffmpegCmd.Stdout = pw
	sinkCmd.Stdin = pr

	var ffmpegStderr strings.Builder
	ffmpegCmd.Stderr = &ffmpegStderr

	if err := ffmpegCmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		p.mu.Unlock()
		fmt.Fprintf(os.Stderr, "ffmpeg start: %v\n", err)
		return
	}
	pw.Close()

	if err := sinkCmd.Start(); err != nil {
		pr.Close()
		p.mu.Unlock()
		fmt.Fprintf(os.Stderr, "%s start: %v\n", audioBackend, err)
		return
	}
	pr.Close()

	p.ffmpeg = ffmpegCmd
	p.sink = sinkCmd
	p.mu.Unlock()

	go func() {
		ffmpegCmd.Wait()
		if s := ffmpegStderr.String(); s != "" {
			fmt.Fprintf(os.Stderr, "ffmpeg: %s\n", s)
		}
		sinkCmd.Wait()

		p.mu.Lock()
		natural := !p.stopped
		p.mu.Unlock()

		if natural && onDone != nil {
			onDone()
		}
	}()
}

func (p *Player) TogglePause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ffmpeg == nil || p.ffmpeg.Process == nil {
		return
	}
	if p.paused {
		cont(p.ffmpeg.Process)
		cont(p.sink.Process)
		p.paused = false
	} else {
		suspend(p.ffmpeg.Process)
		suspend(p.sink.Process)
		p.paused = true
	}
}

func (p *Player) IsPaused() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.paused
}

func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stop()
}

func (p *Player) stop() {
	p.stopped = true
	if p.ffmpeg != nil && p.ffmpeg.Process != nil {
		cont(p.ffmpeg.Process)
		p.ffmpeg.Process.Kill()
	}
	if p.sink != nil && p.sink.Process != nil {
		cont(p.sink.Process)
		p.sink.Process.Kill()
	}
	p.ffmpeg = nil
	p.sink = nil
}

func probeAudio(path string) (rate, channels int) {
	rate, channels = 44100, 2
	out, err := exec.Command("ffprobe",
		"-v", "quiet",
		"-select_streams", "a:0",
		"-show_entries", "stream=sample_rate,channels",
		"-of", "csv=p=0",
		path,
	).Output()
	if err != nil {
		return
	}
	var r, c int
	if n, _ := fmt.Sscanf(strings.TrimSpace(string(out)), "%d,%d", &r, &c); n == 2 {
		rate, channels = r, c
	}
	return
}
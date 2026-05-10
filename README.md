# music-tui-go
A TUI music player written in Go

# Instructions

```bash
# Dependencies (if not already installed)
sudo apt install golang ffmpeg pulseaudio-utils

# Build
cd musicplayer
go build -o musicplayer .

# Run (first time, pass your music directory — it gets saved)
./musicplayer ~/Music

# Subsequent runs
./musicplayer
```

**Controls:**
| Key | Action |
|---|---|
| `↑↓` / `jk` | Navigate |
| `enter` / `space` | Play / pause |
| `n` / `→` | Next |
| `p` / `←` | Previous |
| `s` | Stop |
| `/` | Filter |
| `q` | Quit |
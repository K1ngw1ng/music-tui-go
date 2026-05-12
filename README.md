# music-tui-go
A TUI music player written in Go

## Prebuilt Binary

### Debian (and debian-like systems)    

```bash
# Dependencies (if not already installed)
sudo apt install golang ffmpeg pulseaudio-utils

# Run (first time, pass your music directory, it gets saved)
./musicplayer ~/Music

# Subsequent runs
./musicplayer
```


## Build Instructions

### Debian (and debian-like systems)    

```bash
# Dependencies (if not already installed)
sudo apt install golang ffmpeg pulseaudio-utils

# Build
cd musicplayer
go build -o musicplayer .

# Run (first time, pass your music directory, it gets saved)
./musicplayer ~/Music

# Subsequent runs
./musicplayer
```

# Commands and Controls

## Commands
```bash
# Basic usage
./musicplayer
./musicplayer /path/to/music  # scan a specific directory

# Filter
./musicplayer --album "A Matter of Time"
./musicplayer --artist "Laufey"

# Playlists
./musicplayer --create-playlist mylist # create an empty playlist in music_dir
./musicplayer --create-playlist mylist --output ~/mylist.m3u  # custom location
./musicplayer --create-playlist mylist /path/to/folder     # create and pre-populate
./musicplayer --add-to-playlist mylist /path/to/folder     # add folder
./musicplayer --add-to-playlist mylist /path/to/track.mp3  # add single file
./musicplayer --playlist mylist                            # open playlist
```

## Controls:
| Key | Action |
|---|---|
| `↑↓` / `jk` | Navigate |
| `enter` / `space` | Play / pause |
| `n` / `→` | Next |
| `p` / `←` | Previous |
| `s` | Stop |
| `/` | Filter |
| `q` | Quit |
| `a` | Add to List |

# Images

<p float="left">
  <img src="https://github.com/user-attachments/assets/3c4989ca-7bc6-464d-b55d-25f57387e544" width="1000" /> 
</p>

<p float="left">
  <img src="https://github.com/user-attachments/assets/130794b4-81b6-4eb2-8159-e47cb65711a8" width="500" />
  <img src="https://github.com/user-attachments/assets/55ee51ff-b95f-4fc8-af77-dc56f64adb66" width="500" /> 
</p>

<p float="left">
  <img src="https://github.com/user-attachments/assets/a0530e66-29d7-4791-87f0-a59653fb6a58" width="500" />
</p>


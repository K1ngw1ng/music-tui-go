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

## Last.fm Instructions


### 1. Create a Last.fm API application

1. Go to [https://www.last.fm/api/account/create](https://www.last.fm/api/account/create) and log in
2. Fill in the form:
   - **Application name**: anything (e.g. `musicplayer`)
   - **Application description**: anything
   - **Callback URL**: leave blank
3. Submit, you'll be shown an API key and Shared secret. Keep this page open.

### 2. Run the auth command

```sh
musicplayer --lastfm-auth
```

You'll be prompted for your API key and shared secret:

```
Last.fm API key: <paste API key>
Last.fm API secret: <paste shared secret>
```

The program will then print a URL:

```
Open this URL in your browser and approve access:

  https://www.last.fm/api/auth/?api_key=...&token=...

Press Enter when done...
```

Open the URL, click Allow access, return to the terminal, and press Enter.

On success:

```
Last.fm authentication successful! Credentials saved.
```

Credentials are saved to `~/.config/musicplayer/lastfm.json`      

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

# Last.fm

./musicplayer --lastfm-auth
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


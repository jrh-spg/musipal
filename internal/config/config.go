// Package config loads and writes the musipal TOML config file.
package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// IcecastConfig mirrors musipal.config.IcecastConfig.
type IcecastConfig struct {
	Host          string `toml:"host"`
	Port          int    `toml:"port"`
	Mount         string `toml:"mount"`
	Username      string `toml:"username"`
	Password      string `toml:"password"`
	Bitrate       int    `toml:"bitrate"`
	Format        string `toml:"format"`
	Name          string `toml:"name"`
	Description   string `toml:"description"`
	Genre         string `toml:"genre"`
	AdminUsername string `toml:"admin_username"`
	AdminPassword string `toml:"admin_password"`
	SSL           bool   `toml:"ssl"`
}

// SourceURL builds the ffmpeg `icecast://` source URL for this config.
func (c IcecastConfig) SourceURL() string {
	mount := c.Mount
	if mount == "" || mount[0] != '/' {
		mount = "/" + mount
	}
	user := url.QueryEscape(c.Username)
	pw := url.QueryEscape(c.Password)
	auth := ""
	if user != "" || pw != "" {
		auth = fmt.Sprintf("%s:%s@", user, pw)
	}
	return fmt.Sprintf("icecast://%s%s:%d%s", auth, c.Host, c.Port, mount)
}

// Config mirrors musipal.config.Config.
type Config struct {
	LibraryRoot  string
	PlaylistsDir string
	Icecast      IcecastConfig
}

type fileData struct {
	LibraryRoot  string        `toml:"library_root"`
	PlaylistsDir string        `toml:"playlists_dir"`
	Icecast      IcecastConfig `toml:"icecast"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "musipal", "config.toml")
}

func expandUser(p string) string {
	if p == "" {
		return p
	}
	if p == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if len(p) >= 2 && p[:2] == "~/" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

// Load reads (or creates) the config file, applying an optional library
// root override.
func Load(libraryRootOverride string) (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, err
	}
	cfgPath := configPath()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return Config{}, err
	}

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		defaultLibrary := filepath.Join(home, "Music")
		defaultPlaylists := filepath.Join(home, "Music", "Playlists")
		if err := os.MkdirAll(defaultPlaylists, 0o755); err != nil {
			return Config{}, err
		}
		data := struct {
			LibraryRoot  string `toml:"library_root"`
			PlaylistsDir string `toml:"playlists_dir"`
		}{LibraryRoot: defaultLibrary, PlaylistsDir: defaultPlaylists}
		f, err := os.Create(cfgPath)
		if err != nil {
			return Config{}, err
		}
		defer f.Close()
		if err := toml.NewEncoder(f).Encode(data); err != nil {
			return Config{}, err
		}
	}

	var data fileData
	if _, err := toml.DecodeFile(cfgPath, &data); err != nil {
		return Config{}, err
	}

	libraryRoot := expandUser(data.LibraryRoot)
	if libraryRoot == "" {
		libraryRoot = filepath.Join(home, "Music")
	}
	playlistsDir := expandUser(data.PlaylistsDir)
	if playlistsDir == "" {
		playlistsDir = filepath.Join(home, "Music", "Playlists")
	}

	if libraryRootOverride != "" {
		libraryRoot = expandUser(libraryRootOverride)
	}

	if err := os.MkdirAll(playlistsDir, 0o755); err != nil {
		return Config{}, err
	}

	ic := data.Icecast
	if ic.Host == "" {
		ic.Host = "localhost"
	}
	if ic.Port == 0 {
		ic.Port = 8000
	}
	if ic.Mount == "" {
		ic.Mount = "/stream"
	}
	if ic.Username == "" {
		ic.Username = "source"
	}
	if ic.Bitrate == 0 {
		ic.Bitrate = 128
	}
	if ic.Format == "" {
		ic.Format = "mp3"
	}

	return Config{
		LibraryRoot:  libraryRoot,
		PlaylistsDir: playlistsDir,
		Icecast:      ic,
	}, nil
}

// SessionPath returns ~/.config/musipal/last_session.json.
func SessionPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "musipal", "last_session.json")
}

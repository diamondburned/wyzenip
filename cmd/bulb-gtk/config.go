package main

import (
	"log"
	"net/url"
	"os"
	"path/filepath"
)

var configPath string

func init() {
	u, err := os.UserConfigDir()
	if err != nil {
		return
	}

	configPath = filepath.Join(u, "bulb-gtk", "url")
}

func loadURL() *url.URL {
	if configPath == "" {
		return nil
	}

	b, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}

	u, err := url.Parse(string(b))
	if err != nil {
		log.Println("invalid config URL:", err)
		return nil
	}

	return u
}

func saveURL(u *url.URL) {
	if err := os.MkdirAll(filepath.Dir(configPath), os.ModePerm); err != nil {
		log.Println("failed to mkdir -p config:", err)
		return
	}

	if err := os.WriteFile(configPath, []byte(u.Redacted()), os.ModePerm); err != nil {
		log.Println("failed to save config:", err)
		return
	}
}

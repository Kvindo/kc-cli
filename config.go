package main

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultServer = "https://cloud-api.kvindo.com"

type Profile struct {
	Server string `yaml:"server"`
	Token  string `yaml:"token"`
}

func configDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kc", "config")
}

func cacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kc", "cache")
}

func activeProfile() string {
	data, err := os.ReadFile(filepath.Join(configDir(), "active"))
	if err != nil || len(strings.TrimSpace(string(data))) == 0 {
		return "default"
	}
	return strings.TrimSpace(string(data))
}

func setActiveProfile(name string) error {
	if err := os.MkdirAll(configDir(), 0700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir(), "active"), []byte(name), 0600)
}

func loadProfile(name string) Profile {
	data, err := os.ReadFile(filepath.Join(configDir(), name+".yaml"))
	if err != nil {
		return Profile{Server: defaultServer}
	}
	var p Profile
	if err := yaml.Unmarshal(data, &p); err != nil {
		return Profile{Server: defaultServer}
	}
	if p.Server == "" {
		p.Server = defaultServer
	}
	return p
}

func saveProfile(name string, p Profile) error {
	if err := os.MkdirAll(configDir(), 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir(), name+".yaml"), data, 0600)
}

func clearToken(name string) error {
	p := loadProfile(name)
	p.Token = ""
	return saveProfile(name, p)
}

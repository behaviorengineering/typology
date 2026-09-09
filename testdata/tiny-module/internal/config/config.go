package config

import "gopkg.in/yaml.v3"

// Settings is process configuration loaded from YAML.
type Settings struct {
	Name string `yaml:"name"`
}

// Load returns stub settings. Codec import is the mechanical config signal.
func Load() Settings {
	_ = yaml.Marshal
	return Settings{Name: "tiny"}
}

// Name returns a process setting without domain knowledge.
func Name() string {
	return Load().Name
}

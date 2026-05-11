package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// TierConfig defines the configuration for each cache tier, including its name, restore speed, and capacity.
type TierConfig struct {
	Name         string  `yaml:"name"`
	RestoreSpeed float64 `yaml:"restore_speed"` // blocks per second
	Capacity     int     `yaml:"capacity"`
}

// CacheEngineConfig defines the configuration for the cache engine, which consists of multiple tiers.
type CacheEngineConfig struct {
	Tiers               []TierConfig `yaml:"tiers"`
	RestoreMode         string       `yaml:"restore_mode"`           // "greedy" (default) or "qos"
	MaxRecomputeBlocks  int          `yaml:"max_recompute_blocks"`   // energy-efficiency cap: max cached blocks allowed to be force-recomputed; -1 = unlimited
}

// InferenceEngineConfig defines the configuration for the inference engine, including block size and compute speed.
type InferenceEngineConfig struct {
	BlockSize    int     `yaml:"block_size"`
	ComputeSpeed float64 `yaml:"compute_speed"` // blocks per second
}

// Config is the main configuration struct that includes both the inference engine and cache engine configurations.
type Config struct {
	InferenceEngine InferenceEngineConfig `yaml:"inference_engine"`
	CacheEngine     CacheEngineConfig     `yaml:"cache_engine"`
}

// Load reads a YAML configuration file from the specified path and unmarshals it into a Config struct.
// If the file cannot be read or the YAML is invalid, it returns an error.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

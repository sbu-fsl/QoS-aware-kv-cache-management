package config

// Default returns a Config struct with default values for the inference engine and cache engine configurations.
func Default() *Config {
	return &Config{
		InferenceEngine: InferenceEngineConfig{
			BlockSize:    16,
			ComputeSpeed: 2.0,
		},
		CacheEngine: CacheEngineConfig{
			RestoreMode:        "greedy",
			MaxRecomputeBlocks: -1,
			Tiers: []TierConfig{
				{Name: "VRAM", RestoreSpeed: 100.0, Capacity: 10},
				{Name: "DRAM", RestoreSpeed: 20.0, Capacity: 30},
				{Name: "Disk", RestoreSpeed: 5.0, Capacity: 100},
			},
		},
	}
}

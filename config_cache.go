package main

import "sync"

// ConfigCache provides thread-safe access to the global config
type ConfigCache struct {
	sync.RWMutex
	config *Config
}

var configCache = &ConfigCache{}

// Set stores a config in the cache
func (c *ConfigCache) Set(config *Config) {
	c.Lock()
	defer c.Unlock()
	c.config = config
}

// Get retrieves the config from the cache
func (c *ConfigCache) Get() (*Config, bool) {
	c.RLock()
	defer c.RUnlock()
	return c.config, c.config != nil
}

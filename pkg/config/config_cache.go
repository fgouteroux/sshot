package config

import (
	"sync"

	"github.com/fgouteroux/sshot/pkg/types"
)

// ConfigCache provides thread-safe access to the global config
type ConfigCache struct {
	sync.RWMutex
	config *types.Config
}

var Cache = &ConfigCache{}

// Set stores a config in the cache
func (c *ConfigCache) Set(config *types.Config) {
	c.Lock()
	defer c.Unlock()
	c.config = config
}

// Get retrieves the config from the cache
func (c *ConfigCache) Get() (*types.Config, bool) {
	c.RLock()
	defer c.RUnlock()
	return c.config, c.config != nil
}

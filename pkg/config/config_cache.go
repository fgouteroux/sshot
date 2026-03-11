package config

import (
	"sync"

	"github.com/fgouteroux/sshot/pkg/types"
)

// cache provides thread-safe access to the global config
type cache struct {
	sync.RWMutex
	config *types.Config
}

// Cache is the global config cache instance
var Cache = &cache{}

// Set stores a config in the cache
func (c *cache) Set(config *types.Config) {
	c.Lock()
	defer c.Unlock()
	c.config = config
}

// Get retrieves the config from the cache
func (c *cache) Get() (*types.Config, bool) {
	c.RLock()
	defer c.RUnlock()
	return c.config, c.config != nil
}

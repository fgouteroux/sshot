package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func unmarshalConfig(data []byte, config *Config) error {
	return yaml.Unmarshal(data, config)
}

func unmarshalInventory(data []byte) (*Inventory, error) {
	var invConfig InventoryConfig
	if err := yaml.Unmarshal(data, &invConfig); err != nil {
		return nil, err
	}

	return &Inventory{
		Hosts:     invConfig.Hosts,
		Groups:    invConfig.Groups,
		SSHConfig: invConfig.SSHConfig,
	}, nil
}

func unmarshalPlaybook(data []byte) (*Playbook, error) {
	var pbConfig PlaybookConfig
	if err := yaml.Unmarshal(data, &pbConfig); err != nil {
		return nil, err
	}

	return &Playbook{
		Name:     pbConfig.Name,
		Parallel: pbConfig.Parallel,
		Facts:    pbConfig.Facts,
		Tasks:    pbConfig.Tasks,
	}, nil
}

func loadConfig(playbookPath, inventoryPath string) (*Config, error) {
	// If inventory path is provided, load separate files
	if inventoryPath != "" {
		return loadSeparateFiles(playbookPath, inventoryPath)
	}

	// Otherwise, try to load combined file (legacy format)
	return loadCombinedFile(playbookPath)
}

func loadSeparateFiles(playbookPath, inventoryPath string) (*Config, error) {
	// Load inventory
	invData, err := os.ReadFile(filepath.Clean(inventoryPath))
	if err != nil {
		return nil, fmt.Errorf("failed to read inventory file: %w", err)
	}

	inventory, err := unmarshalInventory(invData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse inventory: %w", err)
	}

	// Load playbook
	pbData, err := os.ReadFile(filepath.Clean(playbookPath))
	if err != nil {
		return nil, fmt.Errorf("failed to read playbook file: %w", err)
	}

	playbook, err := unmarshalPlaybook(pbData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse playbook: %w", err)
	}

	return &Config{
		Inventory: *inventory,
		Playbook:  *playbook,
	}, nil
}

func loadCombinedFile(playbookPath string) (*Config, error) {
	data, err := os.ReadFile(filepath.Clean(playbookPath))
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := unmarshalConfig(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

func applySSHDefaults(config *Config) {
	// Apply defaults to direct hosts
	for i := range config.Inventory.Hosts {
		if config.Inventory.SSHConfig != nil {
			applySSHDefaultsToHost(&config.Inventory.Hosts[i], config.Inventory.SSHConfig)
		} else {
			// No ssh_config, just ensure secure defaults
			ensureSecureDefaults(&config.Inventory.Hosts[i])
		}
	}

	// Apply defaults to group hosts
	for i := range config.Inventory.Groups {
		for j := range config.Inventory.Groups[i].Hosts {
			if config.Inventory.SSHConfig != nil {
				applySSHDefaultsToHost(&config.Inventory.Groups[i].Hosts[j], config.Inventory.SSHConfig)
			} else {
				// No ssh_config, just ensure secure defaults
				ensureSecureDefaults(&config.Inventory.Groups[i].Hosts[j])
			}
		}
	}
}

func ensureSecureDefaults(host *Host) {
	// Set name if not set
	if host.Name == "" {
		if host.Hostname != "" {
			host.Name = host.Hostname
		} else if host.Address != "" {
			host.Name = host.Address
		}
	}

	// Ensure StrictHostKeyCheck has a value (default to true for security)
	if host.StrictHostKeyCheck == nil {
		trueVal := true
		host.StrictHostKeyCheck = &trueVal
	}
}

func applySSHDefaultsToHost(host *Host, defaults *SSHConfig) {
	// Set name to hostname if name is empty
	if host.Name == "" {
		if host.Hostname != "" {
			host.Name = host.Hostname
		} else if host.Address != "" {
			host.Name = host.Address
		}
	}

	// Only apply defaults if host doesn't have its own value
	if host.User == "" && defaults.User != "" {
		host.User = defaults.User
	}
	if host.Password == "" && defaults.Password != "" {
		host.Password = defaults.Password
	}
	if host.KeyFile == "" && defaults.KeyFile != "" {
		host.KeyFile = defaults.KeyFile
	}
	if host.KeyPassword == "" && defaults.KeyPassword != "" {
		host.KeyPassword = defaults.KeyPassword
	}
	if !host.UseAgent && defaults.UseAgent {
		host.UseAgent = defaults.UseAgent
	}
	if host.Port == 0 && defaults.Port != 0 {
		host.Port = defaults.Port
	}

	// Apply strict host key check logic
	// Host-level setting takes precedence if explicitly set
	if host.StrictHostKeyCheck == nil {
		if defaults.StrictHostKeyCheck != nil {
			// Use the default from ssh_config
			host.StrictHostKeyCheck = defaults.StrictHostKeyCheck
		} else {
			// If neither is set, default to true for security
			trueVal := true
			host.StrictHostKeyCheck = &trueVal
		}
	}
	// If host has explicit value, it's already set and we don't override it
}

package main

import (
	"gopkg.in/yaml.v3"
)

func unmarshalConfig(data []byte, config *Config) error {
	return yaml.Unmarshal(data, config)
}

func applySSHDefaults(config *Config) {
	if config.Inventory.SSHConfig == nil {
		return
	}

	sshDefaults := config.Inventory.SSHConfig

	// Apply to direct hosts
	for i := range config.Inventory.Hosts {
		applySSHDefaultsToHost(&config.Inventory.Hosts[i], sshDefaults)
	}

	// Apply to group hosts
	for i := range config.Inventory.Groups {
		for j := range config.Inventory.Groups[i].Hosts {
			applySSHDefaultsToHost(&config.Inventory.Groups[i].Hosts[j], sshDefaults)
		}
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
	// Apply strict host key check default (default is true for security)
	if !host.StrictHostKeyCheck && defaults.StrictHostKeyCheck {
		host.StrictHostKeyCheck = defaults.StrictHostKeyCheck
	}
	// If neither host nor defaults specify, default to true (secure by default)
	if !host.StrictHostKeyCheck && !defaults.StrictHostKeyCheck {
		host.StrictHostKeyCheck = true
	}
}
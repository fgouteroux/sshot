package config

import (
	"github.com/fgouteroux/sshot/pkg/types"
	"testing"
)

func TestUnmarshalConfig(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name: "valid config",
			data: `
inventory:
  hosts:
    - name: test1
      address: 192.168.1.1
      user: testuser
playbook:
  name: Test Playbook
  tasks:
    - name: Test Task
      command: echo test
`,
			wantErr: false,
		},
		{
			name:    "invalid yaml",
			data:    `invalid: [yaml}`,
			wantErr: true,
		},
		{
			name:    "empty config",
			data:    ``,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config types.Config
			err := unmarshalConfig([]byte(tt.data), &config)
			if (err != nil) != tt.wantErr {
				t.Errorf("unmarshalConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestApplySSHDefaults(t *testing.T) {
	tests := []struct {
		name     string
		config   types.Config
		expected types.Config
	}{
		{
			name: "apply defaults to hosts",
			config: types.Config{
				Inventory: types.Inventory{
					SSHConfig: &types.SSHConfig{
						User:               "defaultuser",
						KeyFile:            "/default/key",
						Port:               2222,
						UseAgent:           true,
						StrictHostKeyCheck: types.BoolPtr(true),
					},
					Hosts: []types.Host{
						{
							Address: "192.168.1.1",
						},
					},
				},
			},
			expected: types.Config{
				Inventory: types.Inventory{
					SSHConfig: &types.SSHConfig{
						User:               "defaultuser",
						KeyFile:            "/default/key",
						Port:               2222,
						UseAgent:           true,
						StrictHostKeyCheck: types.BoolPtr(true),
					},
					Hosts: []types.Host{
						{
							Name:               "192.168.1.1",
							Address:            "192.168.1.1",
							User:               "defaultuser",
							KeyFile:            "/default/key",
							Port:               2222,
							UseAgent:           true,
							StrictHostKeyCheck: types.BoolPtr(true),
						},
					},
				},
			},
		},
		{
			name: "host overrides defaults",
			config: types.Config{
				Inventory: types.Inventory{
					SSHConfig: &types.SSHConfig{
						User:               "defaultuser",
						KeyFile:            "/default/key",
						Port:               2222,
						StrictHostKeyCheck: types.BoolPtr(true),
					},
					Hosts: []types.Host{
						{
							Name:    "custom",
							Address: "192.168.1.1",
							User:    "customuser",
							Port:    3333,
						},
					},
				},
			},
			expected: types.Config{
				Inventory: types.Inventory{
					SSHConfig: &types.SSHConfig{
						User:               "defaultuser",
						KeyFile:            "/default/key",
						Port:               2222,
						StrictHostKeyCheck: types.BoolPtr(true),
					},
					Hosts: []types.Host{
						{
							Name:               "custom",
							Address:            "192.168.1.1",
							User:               "customuser",
							KeyFile:            "/default/key",
							Port:               3333,
							StrictHostKeyCheck: types.BoolPtr(true),
						},
					},
				},
			},
		},
		{
			name: "apply defaults to group hosts",
			config: types.Config{
				Inventory: types.Inventory{
					SSHConfig: &types.SSHConfig{
						User:               "defaultuser",
						Port:               2222,
						StrictHostKeyCheck: types.BoolPtr(true),
					},
					Groups: []types.Group{
						{
							Name: "testgroup",
							Hosts: []types.Host{
								{
									Hostname: "host1.example.com",
								},
							},
						},
					},
				},
			},
			expected: types.Config{
				Inventory: types.Inventory{
					SSHConfig: &types.SSHConfig{
						User:               "defaultuser",
						Port:               2222,
						StrictHostKeyCheck: types.BoolPtr(true),
					},
					Groups: []types.Group{
						{
							Name: "testgroup",
							Hosts: []types.Host{
								{
									Name:               "host1.example.com",
									Hostname:           "host1.example.com",
									User:               "defaultuser",
									Port:               2222,
									StrictHostKeyCheck: types.BoolPtr(true),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "no ssh config",
			config: types.Config{
				Inventory: types.Inventory{
					Hosts: []types.Host{
						{
							Name:    "test",
							Address: "192.168.1.1",
							User:    "testuser",
						},
					},
				},
			},
			expected: types.Config{
				Inventory: types.Inventory{
					Hosts: []types.Host{
						{
							Name:               "test",
							Address:            "192.168.1.1",
							User:               "testuser",
							StrictHostKeyCheck: types.BoolPtr(true), // Defaults to true when no ssh_config
						},
					},
				},
			},
		},
		{
			name: "ssh_config sets false, host doesn't override",
			config: types.Config{
				Inventory: types.Inventory{
					SSHConfig: &types.SSHConfig{
						User:               "admin",
						StrictHostKeyCheck: types.BoolPtr(false),
					},
					Hosts: []types.Host{
						{
							Address: "192.168.1.1",
						},
					},
				},
			},
			expected: types.Config{
				Inventory: types.Inventory{
					SSHConfig: &types.SSHConfig{
						User:               "admin",
						StrictHostKeyCheck: types.BoolPtr(false),
					},
					Hosts: []types.Host{
						{
							Name:               "192.168.1.1",
							Address:            "192.168.1.1",
							User:               "admin",
							StrictHostKeyCheck: types.BoolPtr(false),
						},
					},
				},
			},
		},
		{
			name: "ssh_config sets false, host overrides to true",
			config: types.Config{
				Inventory: types.Inventory{
					SSHConfig: &types.SSHConfig{
						User:               "admin",
						StrictHostKeyCheck: types.BoolPtr(false),
					},
					Hosts: []types.Host{
						{
							Address:            "192.168.1.1",
							StrictHostKeyCheck: types.BoolPtr(true),
						},
					},
				},
			},
			expected: types.Config{
				Inventory: types.Inventory{
					SSHConfig: &types.SSHConfig{
						User:               "admin",
						StrictHostKeyCheck: types.BoolPtr(false),
					},
					Hosts: []types.Host{
						{
							Name:               "192.168.1.1",
							Address:            "192.168.1.1",
							User:               "admin",
							StrictHostKeyCheck: types.BoolPtr(true), // types.Host override takes precedence
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ApplySSHDefaults(&tt.config)

			// Compare hosts
			if len(tt.config.Inventory.Hosts) != len(tt.expected.Inventory.Hosts) {
				t.Errorf("host count mismatch: got %d, want %d",
					len(tt.config.Inventory.Hosts), len(tt.expected.Inventory.Hosts))
				return
			}

			for i, host := range tt.config.Inventory.Hosts {
				expected := tt.expected.Inventory.Hosts[i]
				if host.Name != expected.Name {
					t.Errorf("types.Host[%d].Name = %q, want %q", i, host.Name, expected.Name)
				}
				if host.User != expected.User {
					t.Errorf("types.Host[%d].User = %q, want %q", i, host.User, expected.User)
				}
				if host.Port != expected.Port {
					t.Errorf("types.Host[%d].Port = %d, want %d", i, host.Port, expected.Port)
				}
				if host.KeyFile != expected.KeyFile {
					t.Errorf("types.Host[%d].KeyFile = %q, want %q", i, host.KeyFile, expected.KeyFile)
				}
				if host.UseAgent != expected.UseAgent {
					t.Errorf("types.Host[%d].UseAgent = %v, want %v", i, host.UseAgent, expected.UseAgent)
				}
				// Compare StrictHostKeyCheck properly
				if !types.CompareBoolPtr(host.StrictHostKeyCheck, expected.StrictHostKeyCheck) {
					t.Errorf("types.Host[%d].StrictHostKeyCheck = %v, want %v",
						i, types.FormatBoolPtr(host.StrictHostKeyCheck), types.FormatBoolPtr(expected.StrictHostKeyCheck))
				}
			}

			// Compare group hosts
			for i, group := range tt.config.Inventory.Groups {
				expectedGroup := tt.expected.Inventory.Groups[i]
				for j, host := range group.Hosts {
					expected := expectedGroup.Hosts[j]
					if host.Name != expected.Name {
						t.Errorf("types.Group[%d].types.Host[%d].Name = %q, want %q", i, j, host.Name, expected.Name)
					}
					if host.User != expected.User {
						t.Errorf("types.Group[%d].types.Host[%d].User = %q, want %q", i, j, host.User, expected.User)
					}
				}
			}
		})
	}
}

func TestApplySSHDefaultsToHost(t *testing.T) {
	tests := []struct {
		name     string
		host     types.Host
		defaults types.SSHConfig
		expected types.Host
	}{
		{
			name: "set name from hostname",
			host: types.Host{
				Hostname: "server1.example.com",
			},
			defaults: types.SSHConfig{
				User:               "admin",
				StrictHostKeyCheck: types.BoolPtr(true),
			},
			expected: types.Host{
				Name:               "server1.example.com",
				Hostname:           "server1.example.com",
				User:               "admin",
				StrictHostKeyCheck: types.BoolPtr(true),
			},
		},
		{
			name: "set name from address",
			host: types.Host{
				Address: "192.168.1.1",
			},
			defaults: types.SSHConfig{
				StrictHostKeyCheck: types.BoolPtr(true),
			},
			expected: types.Host{
				Name:               "192.168.1.1",
				Address:            "192.168.1.1",
				StrictHostKeyCheck: types.BoolPtr(true),
			},
		},
		{
			name: "preserve existing name",
			host: types.Host{
				Name:     "custom-name",
				Address:  "192.168.1.1",
				Hostname: "server1.example.com",
			},
			defaults: types.SSHConfig{
				StrictHostKeyCheck: types.BoolPtr(true),
			},
			expected: types.Host{
				Name:               "custom-name",
				Address:            "192.168.1.1",
				Hostname:           "server1.example.com",
				StrictHostKeyCheck: types.BoolPtr(true),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applySSHDefaultsToHost(&tt.host, &tt.defaults)

			if tt.host.Name != tt.expected.Name {
				t.Errorf("Name = %q, want %q", tt.host.Name, tt.expected.Name)
			}
			if tt.host.User != tt.expected.User {
				t.Errorf("User = %q, want %q", tt.host.User, tt.expected.User)
			}
			if !types.CompareBoolPtr(tt.host.StrictHostKeyCheck, tt.expected.StrictHostKeyCheck) {
				t.Errorf("StrictHostKeyCheck = %v, want %v",
					types.FormatBoolPtr(tt.host.StrictHostKeyCheck), types.FormatBoolPtr(tt.expected.StrictHostKeyCheck))
			}
		})
	}
}

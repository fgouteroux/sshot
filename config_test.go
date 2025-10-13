package main

import (
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
			var config Config
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
		config   Config
		expected Config
	}{
		{
			name: "apply defaults to hosts",
			config: Config{
				Inventory: Inventory{
					SSHConfig: &SSHConfig{
						User:               "defaultuser",
						KeyFile:            "/default/key",
						Port:               2222,
						UseAgent:           true,
						StrictHostKeyCheck: boolPtr(true),
					},
					Hosts: []Host{
						{
							Address: "192.168.1.1",
						},
					},
				},
			},
			expected: Config{
				Inventory: Inventory{
					SSHConfig: &SSHConfig{
						User:               "defaultuser",
						KeyFile:            "/default/key",
						Port:               2222,
						UseAgent:           true,
						StrictHostKeyCheck: boolPtr(true),
					},
					Hosts: []Host{
						{
							Name:               "192.168.1.1",
							Address:            "192.168.1.1",
							User:               "defaultuser",
							KeyFile:            "/default/key",
							Port:               2222,
							UseAgent:           true,
							StrictHostKeyCheck: boolPtr(true),
						},
					},
				},
			},
		},
		{
			name: "host overrides defaults",
			config: Config{
				Inventory: Inventory{
					SSHConfig: &SSHConfig{
						User:               "defaultuser",
						KeyFile:            "/default/key",
						Port:               2222,
						StrictHostKeyCheck: boolPtr(true),
					},
					Hosts: []Host{
						{
							Name:    "custom",
							Address: "192.168.1.1",
							User:    "customuser",
							Port:    3333,
						},
					},
				},
			},
			expected: Config{
				Inventory: Inventory{
					SSHConfig: &SSHConfig{
						User:               "defaultuser",
						KeyFile:            "/default/key",
						Port:               2222,
						StrictHostKeyCheck: boolPtr(true),
					},
					Hosts: []Host{
						{
							Name:               "custom",
							Address:            "192.168.1.1",
							User:               "customuser",
							KeyFile:            "/default/key",
							Port:               3333,
							StrictHostKeyCheck: boolPtr(true),
						},
					},
				},
			},
		},
		{
			name: "apply defaults to group hosts",
			config: Config{
				Inventory: Inventory{
					SSHConfig: &SSHConfig{
						User:               "defaultuser",
						Port:               2222,
						StrictHostKeyCheck: boolPtr(true),
					},
					Groups: []Group{
						{
							Name: "testgroup",
							Hosts: []Host{
								{
									Hostname: "host1.example.com",
								},
							},
						},
					},
				},
			},
			expected: Config{
				Inventory: Inventory{
					SSHConfig: &SSHConfig{
						User:               "defaultuser",
						Port:               2222,
						StrictHostKeyCheck: boolPtr(true),
					},
					Groups: []Group{
						{
							Name: "testgroup",
							Hosts: []Host{
								{
									Name:               "host1.example.com",
									Hostname:           "host1.example.com",
									User:               "defaultuser",
									Port:               2222,
									StrictHostKeyCheck: boolPtr(true),
								},
							},
						},
					},
				},
			},
		},
		{
			name: "no ssh config",
			config: Config{
				Inventory: Inventory{
					Hosts: []Host{
						{
							Name:    "test",
							Address: "192.168.1.1",
							User:    "testuser",
						},
					},
				},
			},
			expected: Config{
				Inventory: Inventory{
					Hosts: []Host{
						{
							Name:               "test",
							Address:            "192.168.1.1",
							User:               "testuser",
							StrictHostKeyCheck: boolPtr(true), // Defaults to true when no ssh_config
						},
					},
				},
			},
		},
		{
			name: "ssh_config sets false, host doesn't override",
			config: Config{
				Inventory: Inventory{
					SSHConfig: &SSHConfig{
						User:               "admin",
						StrictHostKeyCheck: boolPtr(false),
					},
					Hosts: []Host{
						{
							Address: "192.168.1.1",
						},
					},
				},
			},
			expected: Config{
				Inventory: Inventory{
					SSHConfig: &SSHConfig{
						User:               "admin",
						StrictHostKeyCheck: boolPtr(false),
					},
					Hosts: []Host{
						{
							Name:               "192.168.1.1",
							Address:            "192.168.1.1",
							User:               "admin",
							StrictHostKeyCheck: boolPtr(false),
						},
					},
				},
			},
		},
		{
			name: "ssh_config sets false, host overrides to true",
			config: Config{
				Inventory: Inventory{
					SSHConfig: &SSHConfig{
						User:               "admin",
						StrictHostKeyCheck: boolPtr(false),
					},
					Hosts: []Host{
						{
							Address:            "192.168.1.1",
							StrictHostKeyCheck: boolPtr(true),
						},
					},
				},
			},
			expected: Config{
				Inventory: Inventory{
					SSHConfig: &SSHConfig{
						User:               "admin",
						StrictHostKeyCheck: boolPtr(false),
					},
					Hosts: []Host{
						{
							Name:               "192.168.1.1",
							Address:            "192.168.1.1",
							User:               "admin",
							StrictHostKeyCheck: boolPtr(true), // Host override takes precedence
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applySSHDefaults(&tt.config)

			// Compare hosts
			if len(tt.config.Inventory.Hosts) != len(tt.expected.Inventory.Hosts) {
				t.Errorf("host count mismatch: got %d, want %d",
					len(tt.config.Inventory.Hosts), len(tt.expected.Inventory.Hosts))
				return
			}

			for i, host := range tt.config.Inventory.Hosts {
				expected := tt.expected.Inventory.Hosts[i]
				if host.Name != expected.Name {
					t.Errorf("Host[%d].Name = %q, want %q", i, host.Name, expected.Name)
				}
				if host.User != expected.User {
					t.Errorf("Host[%d].User = %q, want %q", i, host.User, expected.User)
				}
				if host.Port != expected.Port {
					t.Errorf("Host[%d].Port = %d, want %d", i, host.Port, expected.Port)
				}
				if host.KeyFile != expected.KeyFile {
					t.Errorf("Host[%d].KeyFile = %q, want %q", i, host.KeyFile, expected.KeyFile)
				}
				if host.UseAgent != expected.UseAgent {
					t.Errorf("Host[%d].UseAgent = %v, want %v", i, host.UseAgent, expected.UseAgent)
				}
				// Compare StrictHostKeyCheck properly
				if !compareBoolPtr(host.StrictHostKeyCheck, expected.StrictHostKeyCheck) {
					t.Errorf("Host[%d].StrictHostKeyCheck = %v, want %v",
						i, formatBoolPtr(host.StrictHostKeyCheck), formatBoolPtr(expected.StrictHostKeyCheck))
				}
			}

			// Compare group hosts
			for i, group := range tt.config.Inventory.Groups {
				expectedGroup := tt.expected.Inventory.Groups[i]
				for j, host := range group.Hosts {
					expected := expectedGroup.Hosts[j]
					if host.Name != expected.Name {
						t.Errorf("Group[%d].Host[%d].Name = %q, want %q", i, j, host.Name, expected.Name)
					}
					if host.User != expected.User {
						t.Errorf("Group[%d].Host[%d].User = %q, want %q", i, j, host.User, expected.User)
					}
				}
			}
		})
	}
}

func TestApplySSHDefaultsToHost(t *testing.T) {
	tests := []struct {
		name     string
		host     Host
		defaults SSHConfig
		expected Host
	}{
		{
			name: "set name from hostname",
			host: Host{
				Hostname: "server1.example.com",
			},
			defaults: SSHConfig{
				User:               "admin",
				StrictHostKeyCheck: boolPtr(true),
			},
			expected: Host{
				Name:               "server1.example.com",
				Hostname:           "server1.example.com",
				User:               "admin",
				StrictHostKeyCheck: boolPtr(true),
			},
		},
		{
			name: "set name from address",
			host: Host{
				Address: "192.168.1.1",
			},
			defaults: SSHConfig{
				StrictHostKeyCheck: boolPtr(true),
			},
			expected: Host{
				Name:               "192.168.1.1",
				Address:            "192.168.1.1",
				StrictHostKeyCheck: boolPtr(true),
			},
		},
		{
			name: "preserve existing name",
			host: Host{
				Name:     "custom-name",
				Address:  "192.168.1.1",
				Hostname: "server1.example.com",
			},
			defaults: SSHConfig{
				StrictHostKeyCheck: boolPtr(true),
			},
			expected: Host{
				Name:               "custom-name",
				Address:            "192.168.1.1",
				Hostname:           "server1.example.com",
				StrictHostKeyCheck: boolPtr(true),
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
			if !compareBoolPtr(tt.host.StrictHostKeyCheck, tt.expected.StrictHostKeyCheck) {
				t.Errorf("StrictHostKeyCheck = %v, want %v",
					formatBoolPtr(tt.host.StrictHostKeyCheck), formatBoolPtr(tt.expected.StrictHostKeyCheck))
			}
		})
	}
}

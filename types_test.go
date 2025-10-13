package main

import (
	"testing"
)

func TestConfig_Structure(t *testing.T) {
	config := Config{
		Inventory: Inventory{
			Hosts: []Host{
				{
					Name:    "test",
					Address: "127.0.0.1",
					User:    "admin",
				},
			},
		},
		Playbook: Playbook{
			Name: "Test Playbook",
			Tasks: []Task{
				{
					Name:    "Test Task",
					Command: "echo test",
				},
			},
		},
	}

	if len(config.Inventory.Hosts) != 1 {
		t.Errorf("Expected 1 host, got %d", len(config.Inventory.Hosts))
	}

	if len(config.Playbook.Tasks) != 1 {
		t.Errorf("Expected 1 task, got %d", len(config.Playbook.Tasks))
	}

	if config.Inventory.Hosts[0].Name != "test" {
		t.Errorf("Host name = %q, want 'test'", config.Inventory.Hosts[0].Name)
	}

	if config.Playbook.Name != "Test Playbook" {
		t.Errorf("Playbook name = %q, want 'Test Playbook'", config.Playbook.Name)
	}
}

func TestHost_Structure(t *testing.T) {
	host := Host{
		Name:     "server1",
		Address:  "192.168.1.1",
		Hostname: "server1.example.com",
		Port:     22,
		User:     "admin",
		Password: "secret",
		KeyFile:  "/path/to/key",
		Vars: map[string]string{
			"env": "production",
		},
	}

	if host.Name != "server1" {
		t.Errorf("Name = %q, want 'server1'", host.Name)
	}
	if host.Address != "192.168.1.1" {
		t.Errorf("Address = %q, want '192.168.1.1'", host.Address)
	}
	if host.Port != 22 {
		t.Errorf("Port = %d, want 22", host.Port)
	}
	if host.Vars["env"] != "production" {
		t.Errorf("Vars[env] = %q, want 'production'", host.Vars["env"])
	}
}

func TestTask_Structure(t *testing.T) {
	task := Task{
		Name:    "Install Package",
		Command: "apt-get install nginx",
		Sudo:    true,
		When:    "os == 'ubuntu'",
		Vars: map[string]string{
			"package": "nginx",
		},
		Retries:    3,
		RetryDelay: 5,
		Timeout:    60,
	}

	if task.Name != "Install Package" {
		t.Errorf("Name = %q, want 'Install Package'", task.Name)
	}
	if task.Command != "apt-get install nginx" {
		t.Errorf("Command = %q, want 'apt-get install nginx'", task.Command)
	}
	if !task.Sudo {
		t.Error("Sudo should be true")
	}
	if task.Retries != 3 {
		t.Errorf("Retries = %d, want 3", task.Retries)
	}
	if task.RetryDelay != 5 {
		t.Errorf("RetryDelay = %d, want 5", task.RetryDelay)
	}
	if task.Timeout != 60 {
		t.Errorf("Timeout = %d, want 60", task.Timeout)
	}
}

func TestCopyTask_Structure(t *testing.T) {
	copyTask := CopyTask{
		Src:  "/local/file.txt",
		Dest: "/remote/file.txt",
		Mode: "0644",
	}

	if copyTask.Src != "/local/file.txt" {
		t.Errorf("Src = %q, want '/local/file.txt'", copyTask.Src)
	}
	if copyTask.Dest != "/remote/file.txt" {
		t.Errorf("Dest = %q, want '/remote/file.txt'", copyTask.Dest)
	}
	if copyTask.Mode != "0644" {
		t.Errorf("Mode = %q, want '0644'", copyTask.Mode)
	}
}

func TestGroup_Structure(t *testing.T) {
	group := Group{
		Name:  "webservers",
		Order: 1,
		Hosts: []Host{
			{Name: "web1", Address: "192.168.1.10"},
			{Name: "web2", Address: "192.168.1.11"},
		},
		Parallel:  true,
		DependsOn: []string{"dbservers"},
	}

	if group.Name != "webservers" {
		t.Errorf("Name = %q, want 'webservers'", group.Name)
	}
	if group.Order != 1 {
		t.Errorf("Order = %d, want 1", group.Order)
	}
	if len(group.Hosts) != 2 {
		t.Errorf("Expected 2 hosts, got %d", len(group.Hosts))
	}
	if !group.Parallel {
		t.Error("Parallel should be true")
	}
	if len(group.DependsOn) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(group.DependsOn))
	}
}

func TestSSHConfig_Structure(t *testing.T) {
	sshConfig := SSHConfig{
		User:               "admin",
		Password:           "secret",
		KeyFile:            "~/.ssh/id_rsa",
		KeyPassword:        "keypass",
		UseAgent:           true,
		Port:               2222,
		StrictHostKeyCheck: true,
	}

	if sshConfig.User != "admin" {
		t.Errorf("User = %q, want 'admin'", sshConfig.User)
	}
	if sshConfig.Port != 2222 {
		t.Errorf("Port = %d, want 2222", sshConfig.Port)
	}
	if !sshConfig.UseAgent {
		t.Error("UseAgent should be true")
	}
	if !sshConfig.StrictHostKeyCheck {
		t.Error("StrictHostKeyCheck should be true")
	}
}

func TestExecutionOptions_Structure(t *testing.T) {
	opts := ExecutionOptions{
		DryRun:   true,
		Verbose:  true,
		Progress: true,
		NoColor:  true,
	}

	if !opts.DryRun {
		t.Error("DryRun should be true")
	}
	if !opts.Verbose {
		t.Error("Verbose should be true")
	}
	if !opts.Progress {
		t.Error("Progress should be true")
	}
	if !opts.NoColor {
		t.Error("NoColor should be true")
	}
}

func TestHostResult_Structure(t *testing.T) {
	result := HostResult{
		Host: Host{
			Name:    "server1",
			Address: "192.168.1.1",
		},
		Success: true,
		Error:   nil,
		Output:  "Task completed successfully",
	}

	if result.Host.Name != "server1" {
		t.Errorf("Host.Name = %q, want 'server1'", result.Host.Name)
	}
	if !result.Success {
		t.Error("Success should be true")
	}
	if result.Error != nil {
		t.Errorf("Error should be nil, got %v", result.Error)
	}
	if result.Output != "Task completed successfully" {
		t.Errorf("Output = %q, want 'Task completed successfully'", result.Output)
	}
}

func TestPlaybook_Structure(t *testing.T) {
	playbook := Playbook{
		Name:     "Deploy Application",
		Parallel: true,
		Tasks: []Task{
			{Name: "Task1", Command: "echo 1"},
			{Name: "Task2", Command: "echo 2"},
		},
	}

	if playbook.Name != "Deploy Application" {
		t.Errorf("Name = %q, want 'Deploy Application'", playbook.Name)
	}
	if !playbook.Parallel {
		t.Error("Parallel should be true")
	}
	if len(playbook.Tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(playbook.Tasks))
	}
}

func TestInventory_Structure(t *testing.T) {
	inventory := Inventory{
		Hosts: []Host{
			{Name: "host1"},
		},
		Groups: []Group{
			{Name: "group1"},
		},
		SSHConfig: &SSHConfig{
			User: "admin",
		},
	}

	if len(inventory.Hosts) != 1 {
		t.Errorf("Expected 1 host, got %d", len(inventory.Hosts))
	}
	if len(inventory.Groups) != 1 {
		t.Errorf("Expected 1 group, got %d", len(inventory.Groups))
	}
	if inventory.SSHConfig.User != "admin" {
		t.Errorf("SSHConfig.User = %q, want 'admin'", inventory.SSHConfig.User)
	}
}

package playbook

import (
	"github.com/fgouteroux/sshot/pkg/types"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPrintPlaybookSummary(t *testing.T) {
	tests := []struct {
		name     string
		results  []types.HostResult
		err      error
		duration time.Duration
	}{
		{
			name: "all successful",
			results: []types.HostResult{
				{Host: types.Host{Name: "host1"}, Success: true},
				{Host: types.Host{Name: "host2"}, Success: true},
			},
			err:      nil,
			duration: 30 * time.Second,
		},
		{
			name: "some failures",
			results: []types.HostResult{
				{Host: types.Host{Name: "host1"}, Success: true},
				{Host: types.Host{Name: "host2"}, Success: false, Error: os.ErrNotExist},
			},
			err:      os.ErrNotExist,
			duration: 45 * time.Second,
		},
		{
			name: "all failures",
			results: []types.HostResult{
				{Host: types.Host{Name: "host1"}, Success: false},
				{Host: types.Host{Name: "host2"}, Success: false},
			},
			err:      os.ErrInvalid,
			duration: 20 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			// Just verify it doesn't panic
			printPlaybookSummary(tt.results, tt.duration, tt.err)
		})
	}
}

func TestRunPlaybook_InvalidFile(t *testing.T) {
	err := Run("nonexistent.yml", &types.ExecOptions)
	if err == nil {
		t.Error("Run should fail with non-existent file")
	}
}

func TestRunPlaybook_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "invalid.yml")

	err := os.WriteFile(tmpFile, []byte("invalid: [yaml}"), 0600)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	err = Run(tmpFile, &types.ExecOptions)
	if err == nil {
		t.Error("Run should fail with invalid YAML")
	}
}

func TestRunPlaybook_NoHosts(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "nohosts.yml")

	yamlContent := `
inventory:
  hosts: []
  groups: []
playbook:
  name: No Hosts Test
  tasks:
    - name: Test types.Task
      command: echo test
`

	err := os.WriteFile(tmpFile, []byte(yamlContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	err = Run(tmpFile, &types.ExecOptions)
	if err == nil {
		t.Error("Run should fail with no hosts")
	}
	if !strings.Contains(err.Error(), "no hosts or groups") {
		t.Errorf("Error should mention no hosts, got: %v", err)
	}
}

func TestRunPlaybook_DryRun(t *testing.T) {
	types.ExecOptions.DryRun = true
	defer func() {
		types.ExecOptions.DryRun = false
	}()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "dryrun.yml")

	yamlContent := `
inventory:
  ssh_config:
    user: testuser
    password: testpass
  hosts:
    - name: localhost
      address: 127.0.0.1
playbook:
  name: Dry Run Test
  tasks:
    - name: Echo types.Task
      command: echo "test"
`

	err := os.WriteFile(tmpFile, []byte(yamlContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Dry run should succeed even without actual SSH connection
	err = Run(tmpFile, &types.ExecOptions)
	if err != nil {
		t.Errorf("Run dry run failed: %v", err)
	}
}

func TestExecuteWithGroups_DependencyNotMet(t *testing.T) {
	types.ExecOptions.DryRun = true
	defer func() {
		types.ExecOptions.DryRun = false
	}()

	config := types.Config{
		Inventory: types.Inventory{
			Groups: []types.Group{
				{
					Name:  "group1",
					Order: 1,
					Hosts: []types.Host{
						{Name: "host1", Address: "127.0.0.1", User: "testuser", Password: "testpass"},
					},
				},
				{
					Name:  "group2",
					Order: 2,
					Hosts: []types.Host{
						{Name: "host2", Address: "127.0.0.1", User: "testuser", Password: "testpass"},
					},
				},
			},
		},
		Playbook: types.Playbook{
			Name: "Test Order",
			Tasks: []types.Task{
				{Name: "Task1", Command: "echo test"},
			},
		},
	}

	results, err := executeWithGroups(config)
	if err != nil {
		t.Errorf("executeWithGroups failed: %v", err)
	}

	// Verify all groups executed
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestExecuteHostsSequential(t *testing.T) {
	types.ExecOptions.DryRun = true
	defer func() {
		types.ExecOptions.DryRun = false
	}()

	hosts := []types.Host{
		{Name: "host1", Address: "127.0.0.1", User: "testuser", Password: "testpass"},
		{Name: "host2", Address: "127.0.0.2", User: "testuser", Password: "testpass"},
	}

	tasks := []types.Task{
		{Name: "Task1", Command: "echo test"},
	}

	results := executeHostsSequential(hosts, tasks, "")

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// In dry-run, all should succeed
	for i, result := range results {
		if !result.Success {
			t.Errorf("Result[%d] should succeed in dry-run mode", i)
		}
	}
}

func TestExecuteHostsParallel(t *testing.T) {
	types.ExecOptions.DryRun = true
	defer func() {
		types.ExecOptions.DryRun = false
	}()

	hosts := []types.Host{
		{Name: "host1", Address: "127.0.0.1", User: "testuser", Password: "testpass"},
		{Name: "host2", Address: "127.0.0.2", User: "testuser", Password: "testpass"},
		{Name: "host3", Address: "127.0.0.3", User: "testuser", Password: "testpass"},
	}

	tasks := []types.Task{
		{Name: "Task1", Command: "echo test"},
		{Name: "Task2", Command: "echo test2"},
	}

	results := executeHostsParallel(hosts, tasks, "")

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// In dry-run, all should succeed
	for i, result := range results {
		if !result.Success {
			t.Errorf("Result[%d] should succeed in dry-run mode", i)
		}
	}
}

func TestExecuteOnHost_DryRun(t *testing.T) {
	types.ExecOptions.DryRun = true
	defer func() {
		types.ExecOptions.DryRun = false
	}()

	host := types.Host{
		Name:     "testhost",
		Address:  "127.0.0.1",
		User:     "testuser",
		Password: "testpass",
	}

	tasks := []types.Task{
		{Name: "Task1", Command: "echo hello"},
		{Name: "Task2", Command: "echo world"},
	}

	result := executeOnHost(host, tasks, false, "")

	if !result.Success {
		t.Errorf("executeOnHost should succeed in dry-run, got error: %v", result.Error)
	}

	if result.Host.Name != host.Name {
		t.Errorf("Result host name = %q, want %q", result.Host.Name, host.Name)
	}
	config := types.Config{
		Inventory: types.Inventory{
			Groups: []types.Group{
				{
					Name:      "group1",
					Order:     1,
					DependsOn: []string{"missing_group"},
					Hosts: []types.Host{
						{
							Name:     "host1",
							Address:  "127.0.0.1",
							User:     "testuser",
							Password: "testpass",
						},
					},
				},
			},
		},
		Playbook: types.Playbook{
			Name: "Test",
			Tasks: []types.Task{
				{Name: "Task1", Command: "echo test"},
			},
		},
	}

	_, err := executeWithGroups(config)
	if err == nil {
		t.Error("executeWithGroups should fail with unmet dependency")
	}
	if !strings.Contains(err.Error(), "depends on") {
		t.Errorf("Error should mention dependency, got: %v", err)
	}
}

func TestExecuteWithGroups_SortByOrder(t *testing.T) {
	types.ExecOptions.DryRun = true
	types.ExecOptions.Verbose = false
	defer func() {
		types.ExecOptions.DryRun = false
		types.ExecOptions.Verbose = false
	}()

	config := types.Config{
		Inventory: types.Inventory{
			Groups: []types.Group{
				{
					Name:  "group3",
					Order: 3,
					Hosts: []types.Host{
						{Name: "host3", Address: "127.0.0.1", User: "testuser", Password: "testpass"},
					},
				},
				{
					Name:  "group1",
					Order: 1,
					Hosts: []types.Host{
						{Name: "host1", Address: "127.0.0.1", User: "testuser", Password: "testpass"},
					},
				},
				{
					Name:  "group2",
					Order: 2,
					Hosts: []types.Host{
						{Name: "host2", Address: "127.0.0.1", User: "testuser", Password: "testpass"},
					},
				},
			},
		},
		Playbook: types.Playbook{
			Name: "Test Order",
			Tasks: []types.Task{
				{Name: "Task1", Command: "echo test"},
			},
		},
	}

	results, err := executeWithGroups(config)
	if err != nil {
		t.Errorf("executeWithGroups failed: %v", err)
	}

	// Verify all groups executed
	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}
}

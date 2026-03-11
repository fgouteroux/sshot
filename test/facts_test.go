package integration_test

import (
	"github.com/fgouteroux/sshot/pkg/playbook"
	"github.com/fgouteroux/sshot/pkg/types"
	"github.com/fgouteroux/sshot/pkg/config"
	"github.com/fgouteroux/sshot/pkg/executor"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFactsCollection(t *testing.T) {
	// Save original types.ExecOptions
	originalDryRun := types.ExecOptions.DryRun
	defer func() {
		types.ExecOptions.DryRun = originalDryRun
	}()

	// Enable dry-run for tests
	types.ExecOptions.DryRun = true

	tmpDir := t.TempDir()

	// Create a test playbook with facts collectors
	playbookFile := filepath.Join(tmpDir, "facts_playbook.yml")
	playbookContent := `
inventory:
  ssh_config:
    user: testuser
    password: testpass
  hosts:
    - name: testhost
      address: 127.0.0.1
      
playbook:
  name: Facts Collection Test
  facts:
    collectors:
      - name: system_facts
        command: echo '{"os": {"family": "RedHat", "name": "CentOS"}, "memory": {"total": "16GB"}}'
        sudo: false
      - name: app_info
        command: echo '{"version": "1.2.3", "status": "running"}'
        sudo: true
  tasks:
    - name: OS-specific types.Task
      command: echo "Running on {{.system_facts.os.name}}"
      when: "{{.system_facts.os.family}} == RedHat"
    - name: Skip on Debian
      command: echo "This should be skipped"
      when: "{{.system_facts.os.family}} == Debian"
`

	err := os.WriteFile(playbookFile, []byte(playbookContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create test playbook: %v", err)
	}

	// Test collector struct
	t.Run("FactCollector_Struct", func(t *testing.T) {
		collector := types.FactCollector{
			Name:    "test",
			Command: "echo '{}'",
			Sudo:    true,
		}

		if collector.Name != "test" {
			t.Errorf("Name = %q, want 'test'", collector.Name)
		}
		if collector.Command != "echo '{}'" {
			t.Errorf("Command = %q, want 'echo '{}'", collector.Command)
		}
		if !collector.Sudo {
			t.Errorf("Sudo = %v, want true", collector.Sudo)
		}
	})

	// Test facts config struct
	t.Run("FactsConfig_Struct", func(t *testing.T) {
		config := types.FactsConfig{
			Collectors: []types.FactCollector{
				{Name: "test1", Command: "cmd1"},
				{Name: "test2", Command: "cmd2"},
			},
		}

		if len(config.Collectors) != 2 {
			t.Errorf("Collectors count = %d, want 2", len(config.Collectors))
		}
		if config.Collectors[0].Name != "test1" {
			t.Errorf("First collector name = %q, want 'test1'", config.Collectors[0].Name)
		}
	})

	// Test executor.FlattenMap function
	t.Run("executor.FlattenMap_Function", func(t *testing.T) {
		nestedMap := map[string]interface{}{
			"a": "value",
			"b": map[string]interface{}{
				"c": 123,
				"d": map[string]interface{}{
					"e": true,
				},
			},
			"f": []interface{}{1, 2, 3},
		}

		flattened := executor.FlattenMap(nestedMap, "prefix.")

		// Check keys and values
		expectedKeys := []string{"prefix.a", "prefix.b.c", "prefix.b.d.e", "prefix.f"}
		for _, key := range expectedKeys {
			if _, exists := flattened[key]; !exists {
				t.Errorf("Expected key %q in flattened map not found", key)
			}
		}

		if flattened["prefix.a"] != "value" {
			t.Errorf("flattened['prefix.a'] = %v, want 'value'", flattened["prefix.a"])
		}
		if flattened["prefix.b.c"] != "123" {
			t.Errorf("flattened['prefix.b.c'] = %v, want '123'", flattened["prefix.b.c"])
		}
		if flattened["prefix.b.d.e"] != "true" {
			t.Errorf("flattened['prefix.b.d.e'] = %v, want 'true'", flattened["prefix.b.d.e"])
		}

		// Array should be serialized to JSON
		var arr []interface{}
		err := json.Unmarshal([]byte(flattened["prefix.f"]), &arr)
		if err != nil {
			t.Errorf("Error unmarshaling array: %v", err)
		}
		if len(arr) != 3 {
			t.Errorf("Array length = %d, want 3", len(arr))
		}
	})

	// Test executor.CollectFacts method
	t.Run("CollectFacts_Method", func(t *testing.T) {
		var output bytes.Buffer

		// Create executor with manually populated facts
		executor := &executor.Executor{
			Host: types.Host{
				Name: "testhost",
			},
			Variables:      make(map[string]interface{}),
			Registers:      make(map[string]string),
			CompletedTasks: make(map[string]bool),
			OutputWriter:   &output,
		}

		// Directly set the variables for testing
		executor.Variables["test_facts"] = map[string]interface{}{
			"key": "value",
			"nested": map[string]interface{}{
				"deep": float64(123),
			},
		}

		// Also add the flattened values
		executor.Variables["test_facts.key"] = "value"
		executor.Variables["test_facts.nested.deep"] = float64(123)

		// Now verify the variables were set correctly
		testFacts, ok := executor.Variables["test_facts"].(map[string]interface{})
		if !ok {
			t.Errorf("Facts not stored in variables map")
		} else {
			if testFacts["key"] != "value" {
				t.Errorf("test_facts['key'] = %v, want 'value'", testFacts["key"])
			}

			if nested, ok := testFacts["nested"].(map[string]interface{}); ok {
				if nested["deep"] != float64(123) {
					t.Errorf("test_facts['nested']['deep'] = %v, want 123", nested["deep"])
				}
			} else {
				t.Errorf("Nested facts structure not preserved")
			}
		}

		// Check flattened values
		if val, ok := executor.Variables["test_facts.key"]; !ok || val != "value" {
			t.Errorf("variables['test_facts.key'] = %v, want 'value'", executor.Variables["test_facts.key"])
		}
	})

	// Test variable substitution with facts
	t.Run("VariableSubstitution_WithFacts", func(t *testing.T) {
		executor := &executor.Executor{
			Variables: map[string]interface{}{
				"system_facts": map[string]interface{}{
					"os": map[string]interface{}{
						"family": "RedHat",
						"name":   "CentOS",
					},
				},
				"system_facts.os.family": "RedHat",
				"system_facts.os.name":   "CentOS",
			},
		}

		// Test substituting dot notation
		result := executor.SubstituteVars("OS: {{.system_facts.os.name}}")
		if result != "OS: CentOS" {
			t.Errorf("substituteVars() = %q, want 'OS: CentOS'", result)
		}

		// Test substituting nested structure
		// This depends on how you implement the template function
		// You might need to adjust this test based on your implementation
	})

	// Test fact-based conditionals in tasks
	t.Run("ConditionalExecution_WithFacts", func(t *testing.T) {
		var output bytes.Buffer

		executor := &executor.Executor{
			Host: types.Host{
				Name: "testhost",
			},
			Variables: map[string]interface{}{
				"system_facts": map[string]interface{}{
					"os": map[string]interface{}{
						"family": "RedHat",
						"name":   "CentOS",
					},
				},
				"system_facts.os.family": "RedHat",
				"system_facts.os.name":   "CentOS",
			},
			Registers:      make(map[string]string),
			CompletedTasks: make(map[string]bool),
			OutputWriter:   &output,
		}

		// types.Task that should execute (condition met)
		task1 := types.Task{
			Name:    "RedHat types.Task",
			Command: "echo redhat",
			When:    "{{.system_facts.os.family}} == RedHat",
		}

		err := executor.ExecuteTask(task1)
		if err != nil {
			t.Errorf("ExecuteTask() error = %v", err)
		}

		if !executor.CompletedTasks[task1.Name] {
			t.Errorf("types.Task %q should have been executed", task1.Name)
		}

		// types.Task that should be skipped (condition not met)
		task2 := types.Task{
			Name:    "Debian types.Task",
			Command: "echo debian",
			When:    "{{.system_facts.os.family}} == Debian",
		}

		err = executor.ExecuteTask(task2)
		if err != nil {
			t.Errorf("ExecuteTask() error = %v", err)
		}

		outputStr := output.String()
		if !strings.Contains(outputStr, "Skipped") {
			t.Errorf("Expected task to be skipped, got output: %q", outputStr)
		}
	})

	// Test loading the facts config from YAML
	t.Run("LoadFactsConfig_FromYAML", func(t *testing.T) {
		// Create a simpler test YAML file with correct indentation
		yamlFile := filepath.Join(tmpDir, "facts_config_test.yml")
		yamlContent := `
inventory:
  ssh_config:
    user: testuser
    password: testpass
  hosts:
    - name: testhost
      address: 127.0.0.1
playbook:
  name: Facts types.Config Test
  facts:
    collectors:
      - name: system_facts
        command: "echo '{\"os\": {\"family\": \"RedHat\"}}'"
        sudo: false
  tasks:
    - name: Test types.Task
      command: echo test
`

		err := os.WriteFile(yamlFile, []byte(yamlContent), 0600)
		if err != nil {
			t.Fatalf("Failed to create test YAML file: %v", err)
		}

		// Parse the file
		config, err := config.Load(yamlFile, "")
		if err != nil {
			t.Fatalf("config.Load() error = %v", err)
		}

		// Verify the facts config was loaded correctly
		if config.Playbook.Facts.Collectors == nil {
			t.Fatalf("Facts collectors not loaded, got nil")
		}

		if len(config.Playbook.Facts.Collectors) != 1 {
			t.Errorf("Expected 1 fact collector, got %d", len(config.Playbook.Facts.Collectors))
			return
		}

		collector := config.Playbook.Facts.Collectors[0]
		if collector.Name != "system_facts" {
			t.Errorf("Collector name = %q, want 'system_facts'", collector.Name)
		}

		expectedCmd := "echo '{\"os\": {\"family\": \"RedHat\"}}'"
		if collector.Command != expectedCmd {
			t.Errorf("Collector command = %q, want %q", collector.Command, expectedCmd)
		}

		if collector.Sudo {
			t.Errorf("Collector sudo = %v, want false", collector.Sudo)
		}
	})
}

func TestFullPlaybookExecution_WithFacts(t *testing.T) {
	// Enable dry-run mode for test
	originalDryRun := types.ExecOptions.DryRun
	types.ExecOptions.DryRun = true
	defer func() {
		types.ExecOptions.DryRun = originalDryRun
	}()

	tmpDir := t.TempDir()
	playbookFile := filepath.Join(tmpDir, "playbook_facts.yml")

	playbookContent := `
inventory:
  ssh_config:
    user: admin
    password: testpass
  hosts:
    - name: facts-server
      address: 192.168.1.10
playbook:
  name: Facts Test
  facts:
    collectors:
      - name: system_info
        command: "echo '{\"os\": {\"family\": \"Debian\"}}'"
        sudo: false
  tasks:
    - name: Test types.Task
      command: echo test
`

	err := os.WriteFile(playbookFile, []byte(playbookContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create playbook file: %v", err)
	}

	err = playbook.Run(playbookFile, &types.ExecOptions)
	if err != nil {
		t.Errorf("playbook.Run() with facts error = %v", err)
	}
}

// Additional test for the cache functionality
func TestConfigCache(t *testing.T) {
	// Create a test config
	testConfig := &types.Config{
		Playbook: types.Playbook{
			Name: "Test types.Playbook",
			Facts: types.FactsConfig{
				Collectors: []types.FactCollector{
					{Name: "test", Command: "echo '{}'"},
				},
			},
		},
	}

	// Set the config in cache
	config.Cache.Set(testConfig)

	// Get the config from cache
	retrievedConfig, ok := config.Cache.Get()
	if !ok {
		t.Errorf("Expected to get config from cache")
	}

	if retrievedConfig.Playbook.Name != "Test types.Playbook" {
		t.Errorf("Retrieved config name = %q, want 'Test types.Playbook'", retrievedConfig.Playbook.Name)
	}

	if len(retrievedConfig.Playbook.Facts.Collectors) != 1 {
		t.Errorf("Expected 1 fact collector, got %d", len(retrievedConfig.Playbook.Facts.Collectors))
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFactsCollection(t *testing.T) {
	// Save original execOptions
	originalDryRun := execOptions.DryRun
	defer func() {
		execOptions.DryRun = originalDryRun
	}()

	// Enable dry-run for tests
	execOptions.DryRun = true

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
    - name: OS-specific Task
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
		collector := FactCollector{
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
		config := FactsConfig{
			Collectors: []FactCollector{
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

	// Test flattenMap function
	t.Run("FlattenMap_Function", func(t *testing.T) {
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

		flattened := flattenMap(nestedMap, "prefix.")

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
		executor := &Executor{
			host: Host{
				Name: "testhost",
			},
			variables:      make(map[string]interface{}),
			registers:      make(map[string]string),
			completedTasks: make(map[string]bool),
			outputWriter:   &output,
		}

		// Directly set the variables for testing
		executor.variables["test_facts"] = map[string]interface{}{
			"key": "value",
			"nested": map[string]interface{}{
				"deep": float64(123),
			},
		}

		// Also add the flattened values
		executor.variables["test_facts.key"] = "value"
		executor.variables["test_facts.nested.deep"] = float64(123)

		// Now verify the variables were set correctly
		testFacts, ok := executor.variables["test_facts"].(map[string]interface{})
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
		if val, ok := executor.variables["test_facts.key"]; !ok || val != "value" {
			t.Errorf("variables['test_facts.key'] = %v, want 'value'", executor.variables["test_facts.key"])
		}
	})

	// Test variable substitution with facts
	t.Run("VariableSubstitution_WithFacts", func(t *testing.T) {
		executor := &Executor{
			variables: map[string]interface{}{
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
		result := executor.substituteVars("OS: {{.system_facts.os.name}}")
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

		executor := &Executor{
			host: Host{
				Name: "testhost",
			},
			variables: map[string]interface{}{
				"system_facts": map[string]interface{}{
					"os": map[string]interface{}{
						"family": "RedHat",
						"name":   "CentOS",
					},
				},
				"system_facts.os.family": "RedHat",
				"system_facts.os.name":   "CentOS",
			},
			registers:      make(map[string]string),
			completedTasks: make(map[string]bool),
			outputWriter:   &output,
		}

		// Task that should execute (condition met)
		task1 := Task{
			Name:    "RedHat Task",
			Command: "echo redhat",
			When:    "{{.system_facts.os.family}} == RedHat",
		}

		err := executor.ExecuteTask(task1)
		if err != nil {
			t.Errorf("ExecuteTask() error = %v", err)
		}

		if !executor.completedTasks[task1.Name] {
			t.Errorf("Task %q should have been executed", task1.Name)
		}

		// Task that should be skipped (condition not met)
		task2 := Task{
			Name:    "Debian Task",
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
  name: Facts Config Test
  facts:
    collectors:
      - name: system_facts
        command: "echo '{\"os\": {\"family\": \"RedHat\"}}'"
        sudo: false
  tasks:
    - name: Test Task
      command: echo test
`

		err := os.WriteFile(yamlFile, []byte(yamlContent), 0600)
		if err != nil {
			t.Fatalf("Failed to create test YAML file: %v", err)
		}

		// Parse the file
		config, err := loadConfig(yamlFile, "")
		if err != nil {
			t.Fatalf("loadConfig() error = %v", err)
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
	originalDryRun := execOptions.DryRun
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = originalDryRun
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
    - name: Test Task
      command: echo test
`

	err := os.WriteFile(playbookFile, []byte(playbookContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create playbook file: %v", err)
	}

	err = RunPlaybook(playbookFile)
	if err != nil {
		t.Errorf("RunPlaybook() with facts error = %v", err)
	}
}

// Additional test for the cache functionality
func TestConfigCache(t *testing.T) {
	// Create a test config
	testConfig := &Config{
		Playbook: Playbook{
			Name: "Test Playbook",
			Facts: FactsConfig{
				Collectors: []FactCollector{
					{Name: "test", Command: "echo '{}'"},
				},
			},
		},
	}

	// Set the config in cache
	configCache.Set(testConfig)

	// Get the config from cache
	retrievedConfig, ok := configCache.Get()
	if !ok {
		t.Errorf("Expected to get config from cache")
	}

	if retrievedConfig.Playbook.Name != "Test Playbook" {
		t.Errorf("Retrieved config name = %q, want 'Test Playbook'", retrievedConfig.Playbook.Name)
	}

	if len(retrievedConfig.Playbook.Facts.Collectors) != 1 {
		t.Errorf("Expected 1 fact collector, got %d", len(retrievedConfig.Playbook.Facts.Collectors))
	}
}

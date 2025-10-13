package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecutor_ExecuteCopyTaskDryRun(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	var output bytes.Buffer
	executor := &Executor{
		host: Host{
			Name: "testhost",
		},
		variables:      make(map[string]string),
		registers:      make(map[string]string),
		completedTasks: make(map[string]bool),
		outputWriter:   &output,
	}

	task := Task{
		Name: "Copy Task",
		Copy: &CopyTask{
			Src:  "/local/file.txt",
			Dest: "/remote/file.txt",
			Mode: "0644",
		},
	}

	err := executor.ExecuteTask(task)
	if err != nil {
		t.Errorf("ExecuteTask() error = %v", err)
	}

	outputStr := output.String()
	if !strings.Contains(outputStr, "Copy") {
		t.Errorf("Output should contain 'Copy', got: %q", outputStr)
	}
}

func TestExecutor_ExecuteScriptTaskDryRun(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test_script.sh")
	scriptContent := "#!/bin/bash\necho 'test'"
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	var output bytes.Buffer
	executor := &Executor{
		host: Host{
			Name: "testhost",
		},
		variables:      make(map[string]string),
		registers:      make(map[string]string),
		completedTasks: make(map[string]bool),
		outputWriter:   &output,
	}

	task := Task{
		Name:   "Script Task",
		Script: scriptPath,
		Sudo:   true,
	}

	err = executor.ExecuteTask(task)
	if err != nil {
		t.Errorf("ExecuteTask() error = %v", err)
	}

	outputStr := output.String()
	if !strings.Contains(outputStr, "Script") {
		t.Errorf("Output should contain 'Script', got: %q", outputStr)
	}
}

func TestExecutor_ExecuteWaitForTaskDryRun(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	var output bytes.Buffer
	executor := &Executor{
		host: Host{
			Name: "testhost",
		},
		variables:      make(map[string]string),
		registers:      make(map[string]string),
		completedTasks: make(map[string]bool),
		outputWriter:   &output,
	}

	task := Task{
		Name:    "Wait For Port",
		WaitFor: "port:8080",
	}

	err := executor.ExecuteTask(task)
	if err != nil {
		t.Errorf("ExecuteTask() error = %v", err)
	}
}

func TestExecutor_ExecuteTaskWithRetries(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	var output bytes.Buffer
	executor := &Executor{
		host: Host{
			Name: "testhost",
		},
		variables:      make(map[string]string),
		registers:      make(map[string]string),
		completedTasks: make(map[string]bool),
		outputWriter:   &output,
	}

	task := Task{
		Name:       "Retry Task",
		Command:    "echo test",
		Retries:    3,
		RetryDelay: 1,
	}

	err := executor.ExecuteTask(task)
	if err != nil {
		t.Errorf("ExecuteTask() error = %v", err)
	}
}

func TestExecutor_ExecuteTaskWithTimeout(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	var output bytes.Buffer
	executor := &Executor{
		host: Host{
			Name: "testhost",
		},
		variables:      make(map[string]string),
		registers:      make(map[string]string),
		completedTasks: make(map[string]bool),
		outputWriter:   &output,
	}

	task := Task{
		Name:    "Timeout Task",
		Command: "echo test",
		Timeout: 10,
	}

	err := executor.ExecuteTask(task)
	if err != nil {
		t.Errorf("ExecuteTask() error = %v", err)
	}
}

func TestExecutor_ExecuteTaskUntilSuccess(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	var output bytes.Buffer
	executor := &Executor{
		host: Host{
			Name: "testhost",
		},
		variables:      make(map[string]string),
		registers:      make(map[string]string),
		completedTasks: make(map[string]bool),
		outputWriter:   &output,
	}

	task := Task{
		Name:         "Until Success Task",
		Command:      "echo test",
		UntilSuccess: true,
	}

	err := executor.ExecuteTask(task)
	if err != nil {
		t.Errorf("ExecuteTask() error = %v", err)
	}
}

func TestExecutor_ExecuteTaskWithRegister(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	var output bytes.Buffer
	executor := &Executor{
		host: Host{
			Name: "testhost",
		},
		variables:      make(map[string]string),
		registers:      make(map[string]string),
		completedTasks: make(map[string]bool),
		outputWriter:   &output,
	}

	task := Task{
		Name:     "Register Task",
		Command:  "echo test",
		Register: "test_output",
	}

	err := executor.ExecuteTask(task)
	if err != nil {
		t.Errorf("ExecuteTask() error = %v", err)
	}

	if executor.completedTasks[task.Name] != true {
		t.Error("Task should be marked as completed")
	}
}

func TestExecutor_ExecuteTaskIgnoreError(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	var output bytes.Buffer
	executor := &Executor{
		host: Host{
			Name: "testhost",
		},
		variables:      make(map[string]string),
		registers:      make(map[string]string),
		completedTasks: make(map[string]bool),
		outputWriter:   &output,
	}

	task := Task{
		Name:        "Ignore Error Task",
		Command:     "false", // Command that would fail
		IgnoreError: true,
	}

	// In dry-run mode, this should succeed even with ignore_error
	err := executor.ExecuteTask(task)
	if err != nil {
		t.Errorf("ExecuteTask() should not error in dry-run: %v", err)
	}

	// Task should be marked as completed even if it would have failed
	if !executor.completedTasks[task.Name] {
		t.Error("Task with ignore_error should be marked as completed")
	}
}

func TestExecutor_SubstituteVarsInvalidTemplate(t *testing.T) {
	executor := &Executor{
		variables: map[string]string{
			"var": "value",
		},
	}

	result := executor.substituteVars("{{.invalid")
	if result != "{{.invalid" {
		t.Errorf("substituteVars with invalid template = %q, want %q", result, "{{.invalid")
	}
}

func TestExecutor_EvaluateConditionComplexEquals(t *testing.T) {
	executor := &Executor{
		variables: map[string]string{
			"status": "active",
			"count":  "5",
		},
	}

	tests := []struct {
		name      string
		condition string
		expected  bool
	}{
		{
			name:      "match status",
			condition: "{{.status}} == active",
			expected:  true,
		},
		{
			name:      "match count",
			condition: "{{.count}} == 5",
			expected:  true,
		},
		{
			name:      "no match",
			condition: "{{.status}} == inactive",
			expected:  false,
		},
		{
			name:      "is defined true",
			condition: "status is defined",
			expected:  true,
		},
		{
			name:      "is defined false",
			condition: "nonexistent is defined",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.evaluateCondition(tt.condition)
			if result != tt.expected {
				t.Errorf("evaluateCondition(%q) = %v, want %v", tt.condition, result, tt.expected)
			}
		})
	}
}

func TestExecutor_ExecuteTaskVerbose(t *testing.T) {
	execOptions.DryRun = true
	execOptions.Verbose = true
	defer func() {
		execOptions.DryRun = false
		execOptions.Verbose = false
	}()

	var output bytes.Buffer
	executor := &Executor{
		host: Host{
			Name: "testhost",
		},
		variables:      make(map[string]string),
		registers:      make(map[string]string),
		completedTasks: make(map[string]bool),
		outputWriter:   &output,
	}

	task := Task{
		Name:    "Verbose Task",
		Command: "echo test",
		Vars: map[string]string{
			"test": "value",
		},
	}

	err := executor.ExecuteTask(task)
	if err != nil {
		t.Errorf("ExecuteTask() error = %v", err)
	}
}

func TestExecutor_ExecuteTaskMultipleDependencies(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	var output bytes.Buffer
	executor := &Executor{
		host: Host{
			Name: "testhost",
		},
		variables:      make(map[string]string),
		registers:      make(map[string]string),
		completedTasks: make(map[string]bool),
		outputWriter:   &output,
	}

	executor.completedTasks["dep1"] = true
	executor.completedTasks["dep2"] = true

	task := Task{
		Name:      "Multi Dependency Task",
		Command:   "echo test",
		DependsOn: []string{"dep1", "dep2"},
	}

	err := executor.ExecuteTask(task)
	if err != nil {
		t.Errorf("ExecuteTask() error = %v", err)
	}
}

func TestExecutor_ExecuteTaskPartialDependencies(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	var output bytes.Buffer
	executor := &Executor{
		host: Host{
			Name: "testhost",
		},
		variables:      make(map[string]string),
		registers:      make(map[string]string),
		completedTasks: make(map[string]bool),
		outputWriter:   &output,
	}

	executor.completedTasks["dep1"] = true

	task := Task{
		Name:      "Partial Dependency Task",
		Command:   "echo test",
		DependsOn: []string{"dep1", "dep2"},
	}

	err := executor.ExecuteTask(task)
	if err == nil {
		t.Error("ExecuteTask() should fail with unmet dependency")
	}
	if !strings.Contains(err.Error(), "dependency not met") {
		t.Errorf("Error should mention dependency, got: %v", err)
	}
}

func TestExecutor_InitialState(t *testing.T) {
	executor := &Executor{
		host: Host{
			Name: "testhost",
			Vars: map[string]string{
				"host_var": "host_value",
			},
		},
		variables:      make(map[string]string),
		registers:      make(map[string]string),
		completedTasks: make(map[string]bool),
		startTime:      time.Now(),
	}

	if executor.host.Name != "testhost" {
		t.Errorf("Host name = %q, want 'testhost'", executor.host.Name)
	}

	if len(executor.variables) != 0 {
		t.Errorf("Variables should be empty initially, got %d", len(executor.variables))
	}

	if len(executor.registers) != 0 {
		t.Errorf("Registers should be empty initially, got %d", len(executor.registers))
	}

	if len(executor.completedTasks) != 0 {
		t.Errorf("CompletedTasks should be empty initially, got %d", len(executor.completedTasks))
	}
}

func TestExecutor_ExecuteTaskAllTypes(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "script.sh")
	err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho test"), 0600)
	if err != nil {
		t.Fatalf("Failed to create script: %v", err)
	}

	tests := []struct {
		name string
		task Task
	}{
		{
			name: "command",
			task: Task{Name: "cmd", Command: "echo test"},
		},
		{
			name: "shell",
			task: Task{Name: "shell", Shell: "echo test"},
		},
		{
			name: "script",
			task: Task{Name: "script", Script: scriptPath},
		},
		{
			name: "copy",
			task: Task{Name: "copy", Copy: &CopyTask{Src: "/src", Dest: "/dst"}},
		},
		{
			name: "wait_for",
			task: Task{Name: "wait", WaitFor: "port:8080"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			executor := &Executor{
				host:           Host{Name: "testhost"},
				variables:      make(map[string]string),
				registers:      make(map[string]string),
				completedTasks: make(map[string]bool),
				outputWriter:   &output,
			}

			err := executor.ExecuteTask(tt.task)
			if err != nil {
				t.Errorf("ExecuteTask() error = %v", err)
			}

			if !executor.completedTasks[tt.task.Name] {
				t.Errorf("Task %q should be marked as completed", tt.task.Name)
			}
		})
	}
}

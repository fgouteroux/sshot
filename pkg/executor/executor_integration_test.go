package executor

import "github.com/fgouteroux/sshot/pkg/types"

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecutor_ExecuteCopyTaskDryRun(t *testing.T) {
	types.ExecOptions.DryRun = true
	defer func() {
		types.ExecOptions.DryRun = false
	}()

	var output bytes.Buffer
	executor := &Executor{
		Host: types.Host{
			Name: "testhost",
		},
		Variables:      make(map[string]interface{}),
		Registers:      make(map[string]string),
		CompletedTasks: make(map[string]bool),
		OutputWriter:   &output,
	}

	task := types.Task{
		Name: "Copy types.Task",
		Copy: &types.CopyTask{
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
	types.ExecOptions.DryRun = true
	defer func() {
		types.ExecOptions.DryRun = false
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
		Host: types.Host{
			Name: "testhost",
		},
		Variables:      make(map[string]interface{}),
		Registers:      make(map[string]string),
		CompletedTasks: make(map[string]bool),
		OutputWriter:   &output,
	}

	task := types.Task{
		Name:   "Script types.Task",
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
	types.ExecOptions.DryRun = true
	defer func() {
		types.ExecOptions.DryRun = false
	}()

	var output bytes.Buffer
	executor := &Executor{
		Host: types.Host{
			Name: "testhost",
		},
		Variables:      make(map[string]interface{}),
		Registers:      make(map[string]string),
		CompletedTasks: make(map[string]bool),
		OutputWriter:   &output,
	}

	task := types.Task{
		Name:    "Wait For Port",
		WaitFor: "port:8080",
	}

	err := executor.ExecuteTask(task)
	if err != nil {
		t.Errorf("ExecuteTask() error = %v", err)
	}
}

func TestExecutor_ExecuteTaskWithRetries(t *testing.T) {
	types.ExecOptions.DryRun = true
	defer func() {
		types.ExecOptions.DryRun = false
	}()

	var output bytes.Buffer
	executor := &Executor{
		Host: types.Host{
			Name: "testhost",
		},
		Variables:      make(map[string]interface{}),
		Registers:      make(map[string]string),
		CompletedTasks: make(map[string]bool),
		OutputWriter:   &output,
	}

	task := types.Task{
		Name:       "Retry types.Task",
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
	types.ExecOptions.DryRun = true
	defer func() {
		types.ExecOptions.DryRun = false
	}()

	var output bytes.Buffer
	executor := &Executor{
		Host: types.Host{
			Name: "testhost",
		},
		Variables:      make(map[string]interface{}),
		Registers:      make(map[string]string),
		CompletedTasks: make(map[string]bool),
		OutputWriter:   &output,
	}

	task := types.Task{
		Name:    "Timeout types.Task",
		Command: "echo test",
		Timeout: 10,
	}

	err := executor.ExecuteTask(task)
	if err != nil {
		t.Errorf("ExecuteTask() error = %v", err)
	}
}

func TestExecutor_ExecuteTaskUntilSuccess(t *testing.T) {
	types.ExecOptions.DryRun = true
	defer func() {
		types.ExecOptions.DryRun = false
	}()

	var output bytes.Buffer
	executor := &Executor{
		Host: types.Host{
			Name: "testhost",
		},
		Variables:      make(map[string]interface{}),
		Registers:      make(map[string]string),
		CompletedTasks: make(map[string]bool),
		OutputWriter:   &output,
	}

	task := types.Task{
		Name:         "Until Success types.Task",
		Command:      "echo test",
		UntilSuccess: true,
	}

	err := executor.ExecuteTask(task)
	if err != nil {
		t.Errorf("ExecuteTask() error = %v", err)
	}
}

func TestExecutor_ExecuteTaskWithRegister(t *testing.T) {
	types.ExecOptions.DryRun = true
	defer func() {
		types.ExecOptions.DryRun = false
	}()

	var output bytes.Buffer
	executor := &Executor{
		Host: types.Host{
			Name: "testhost",
		},
		Variables:      make(map[string]interface{}),
		Registers:      make(map[string]string),
		CompletedTasks: make(map[string]bool),
		OutputWriter:   &output,
	}

	task := types.Task{
		Name:     "Register types.Task",
		Command:  "echo test",
		Register: "test_output",
	}

	err := executor.ExecuteTask(task)
	if err != nil {
		t.Errorf("ExecuteTask() error = %v", err)
	}

	if executor.CompletedTasks[task.Name] != true {
		t.Error("types.Task should be marked as completed")
	}
}

func TestExecutor_ExecuteTaskIgnoreError(t *testing.T) {
	types.ExecOptions.DryRun = true
	defer func() {
		types.ExecOptions.DryRun = false
	}()

	var output bytes.Buffer
	executor := &Executor{
		Host: types.Host{
			Name: "testhost",
		},
		Variables:      make(map[string]interface{}),
		Registers:      make(map[string]string),
		CompletedTasks: make(map[string]bool),
		OutputWriter:   &output,
	}

	task := types.Task{
		Name:        "Ignore Error types.Task",
		Command:     "false", // Command that would fail
		IgnoreError: true,
	}

	// In dry-run mode, this should succeed even with ignore_error
	err := executor.ExecuteTask(task)
	if err != nil {
		t.Errorf("ExecuteTask() should not error in dry-run: %v", err)
	}

	// types.Task should be marked as completed even if it would have failed
	if !executor.CompletedTasks[task.Name] {
		t.Error("types.Task with ignore_error should be marked as completed")
	}
}

func TestExecutor_SubstituteVarsInvalidTemplate(t *testing.T) {
	executor := &Executor{
		Variables: map[string]interface{}{
			"var": "value",
		},
	}

	result := executor.SubstituteVars("{{.invalid")
	if result != "{{.invalid" {
		t.Errorf("SubstituteVars with invalid template = %q, want %q", result, "{{.invalid")
	}
}

func TestExecutor_EvaluateConditionComplexEquals(t *testing.T) {
	executor := &Executor{
		Variables: map[string]interface{}{
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
	types.ExecOptions.DryRun = true
	types.ExecOptions.Verbose = true
	defer func() {
		types.ExecOptions.DryRun = false
		types.ExecOptions.Verbose = false
	}()

	var output bytes.Buffer
	executor := &Executor{
		Host: types.Host{
			Name: "testhost",
		},
		Variables:      make(map[string]interface{}),
		Registers:      make(map[string]string),
		CompletedTasks: make(map[string]bool),
		OutputWriter:   &output,
	}

	task := types.Task{
		Name:    "Verbose types.Task",
		Command: "echo test",
		Vars: map[string]interface{}{
			"test": "value",
		},
	}

	err := executor.ExecuteTask(task)
	if err != nil {
		t.Errorf("ExecuteTask() error = %v", err)
	}
}

func TestExecutor_ExecuteTaskMultipleDependencies(t *testing.T) {
	types.ExecOptions.DryRun = true
	defer func() {
		types.ExecOptions.DryRun = false
	}()

	var output bytes.Buffer
	executor := &Executor{
		Host: types.Host{
			Name: "testhost",
		},
		Variables:      make(map[string]interface{}),
		Registers:      make(map[string]string),
		CompletedTasks: make(map[string]bool),
		OutputWriter:   &output,
	}

	executor.CompletedTasks["dep1"] = true
	executor.CompletedTasks["dep2"] = true

	task := types.Task{
		Name:      "Multi Dependency types.Task",
		Command:   "echo test",
		DependsOn: []string{"dep1", "dep2"},
	}

	err := executor.ExecuteTask(task)
	if err != nil {
		t.Errorf("ExecuteTask() error = %v", err)
	}
}

func TestExecutor_ExecuteTaskPartialDependencies(t *testing.T) {
	types.ExecOptions.DryRun = true
	defer func() {
		types.ExecOptions.DryRun = false
	}()

	var output bytes.Buffer
	executor := &Executor{
		Host: types.Host{
			Name: "testhost",
		},
		Variables:      make(map[string]interface{}),
		Registers:      make(map[string]string),
		CompletedTasks: make(map[string]bool),
		OutputWriter:   &output,
	}

	executor.CompletedTasks["dep1"] = true

	task := types.Task{
		Name:      "Partial Dependency types.Task",
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
		Host: types.Host{
			Name: "testhost",
			Vars: map[string]interface{}{
				"host_var": "host_value",
			},
		},
		Variables:      make(map[string]interface{}),
		Registers:      make(map[string]string),
		CompletedTasks: make(map[string]bool),
		StartTime:      time.Now(),
	}

	if executor.Host.Name != "testhost" {
		t.Errorf("types.Host name = %q, want 'testhost'", executor.Host.Name)
	}

	if len(executor.Variables) != 0 {
		t.Errorf("Variables should be empty initially, got %d", len(executor.Variables))
	}

	if len(executor.Registers) != 0 {
		t.Errorf("Registers should be empty initially, got %d", len(executor.Registers))
	}

	if len(executor.CompletedTasks) != 0 {
		t.Errorf("CompletedTasks should be empty initially, got %d", len(executor.CompletedTasks))
	}
}

func TestExecutor_ExecuteTaskAllTypes(t *testing.T) {
	types.ExecOptions.DryRun = true
	defer func() {
		types.ExecOptions.DryRun = false
	}()

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "script.sh")
	err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho test"), 0600)
	if err != nil {
		t.Fatalf("Failed to create script: %v", err)
	}

	tests := []struct {
		name string
		task types.Task
	}{
		{
			name: "command",
			task: types.Task{Name: "cmd", Command: "echo test"},
		},
		{
			name: "shell",
			task: types.Task{Name: "shell", Shell: "echo test"},
		},
		{
			name: "script",
			task: types.Task{Name: "script", Script: scriptPath},
		},
		{
			name: "copy",
			task: types.Task{Name: "copy", Copy: &types.CopyTask{Src: "/src", Dest: "/dst"}},
		},
		{
			name: "wait_for",
			task: types.Task{Name: "wait", WaitFor: "port:8080"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			executor := &Executor{
				Host:           types.Host{Name: "testhost"},
				Variables:      make(map[string]interface{}),
				Registers:      make(map[string]string),
				CompletedTasks: make(map[string]bool),
				OutputWriter:   &output,
			}

			err := executor.ExecuteTask(tt.task)
			if err != nil {
				t.Errorf("ExecuteTask() error = %v", err)
			}

			if !executor.CompletedTasks[tt.task.Name] {
				t.Errorf("types.Task %q should be marked as completed", tt.task.Name)
			}
		})
	}
}

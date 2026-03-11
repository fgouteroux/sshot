package executor

import (
	"github.com/fgouteroux/sshot/pkg/types"
	"bytes"
	"strings"
	"testing"
)

func TestExecutor_SubstituteVars(t *testing.T) {
	executor := &Executor{
		Variables: map[string]interface{}{
			"username": "admin",
			"port":     "8080",
			"path":     "/var/log",
		},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single variable",
			input:    "User: {{.username}}",
			expected: "User: admin",
		},
		{
			name:     "multiple variables",
			input:    "http://localhost:{{.port}}{{.path}}",
			expected: "http://localhost:8080/var/log",
		},
		{
			name:     "no variables",
			input:    "plain text",
			expected: "plain text",
		},
		{
			name:     "undefined variable",
			input:    "Value: {{.undefined}}",
			expected: "Value: <no value>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := executor.SubstituteVars(tt.input)
			if result != tt.expected {
				t.Errorf("substituteVars() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestExecutor_EvaluateCondition(t *testing.T) {
	executor := &Executor{
		Variables: map[string]interface{}{
			"os":      "ubuntu",
			"version": "20.04",
			"defined": "value",
		},
	}

	tests := []struct {
		name      string
		condition string
		expected  bool
	}{
		{
			name:      "variable equals string - match",
			condition: "{{.os}} == ubuntu",
			expected:  true,
		},
		{
			name:      "variable equals string - no match",
			condition: "{{.os}} == centos",
			expected:  false,
		},
		{
			name:      "variable is defined",
			condition: "defined is defined",
			expected:  true,
		},
		{
			name:      "variable is not defined",
			condition: "undefined is defined",
			expected:  false,
		},
		{
			name:      "empty condition",
			condition: "",
			expected:  true,
		},
		{
			name:      "version comparison - match",
			condition: "{{.version}} == 20.04",
			expected:  true,
		},
		{
			name:      "version comparison - no match",
			condition: "{{.version}} == 18.04",
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

func TestExecutor_ExecuteTaskDryRun(t *testing.T) {
	types.ExecOptions.DryRun = true
	types.ExecOptions.Verbose = false
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

	tests := []struct {
		name        string
		task        types.Task
		expectedOut string
	}{
		{
			name: "command task",
			task: types.Task{
				Name:    "Test Command",
				Command: "echo hello",
			},
			expectedOut: "DRY-RUN",
		},
		{
			name: "command with sudo",
			task: types.Task{
				Name:    "Sudo Command",
				Command: "apt-get update",
				Sudo:    true,
			},
			expectedOut: "sudo",
		},
		{
			name: "copy task",
			task: types.Task{
				Name: "Copy File",
				Copy: &types.CopyTask{
					Src:  "/local/file",
					Dest: "/remote/file",
				},
			},
			expectedOut: "Copy",
		},
		{
			name: "script task",
			task: types.Task{
				Name:   "Run Script",
				Script: "/path/to/script.sh",
			},
			expectedOut: "Script",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output.Reset()
			err := executor.ExecuteTask(tt.task)
			if err != nil {
				t.Errorf("ExecuteTask() error = %v", err)
			}
			outputStr := output.String()
			if !strings.Contains(outputStr, tt.expectedOut) {
				t.Errorf("Output should contain %q, got: %q", tt.expectedOut, outputStr)
			}
			if !executor.CompletedTasks[tt.task.Name] {
				t.Errorf("types.Task %q should be marked as completed", tt.task.Name)
			}
		})
	}
}

func TestExecutor_ExecuteTaskWithCondition(t *testing.T) {
	types.ExecOptions.DryRun = true
	defer func() {
		types.ExecOptions.DryRun = false
	}()

	var output bytes.Buffer
	executor := &Executor{
		Host: types.Host{
			Name: "testhost",
		},
		Variables: map[string]interface{}{
			"os": "ubuntu",
		},
		Registers:      make(map[string]string),
		CompletedTasks: make(map[string]bool),
		OutputWriter:   &output,
	}

	tests := []struct {
		name          string
		task          types.Task
		expectSkipped bool
	}{
		{
			name: "condition met",
			task: types.Task{
				Name:    "Ubuntu types.Task",
				Command: "apt-get update",
				When:    "{{.os}} == ubuntu",
			},
			expectSkipped: false,
		},
		{
			name: "condition not met",
			task: types.Task{
				Name:    "CentOS types.Task",
				Command: "yum update",
				When:    "{{.os}} == centos",
			},
			expectSkipped: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output.Reset()
			err := executor.ExecuteTask(tt.task)
			if err != nil {
				t.Errorf("ExecuteTask() error = %v", err)
			}

			outputStr := output.String()
			if tt.expectSkipped {
				if !strings.Contains(outputStr, "Skipped") {
					t.Errorf("Expected task to be skipped, got: %q", outputStr)
				}
			} else {
				if strings.Contains(outputStr, "Skipped") {
					t.Errorf("Expected task to execute, but was skipped")
				}
			}
		})
	}
}

func TestExecutor_ExecuteTaskWithDependencies(t *testing.T) {
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

	// Mark a task as completed
	executor.CompletedTasks["first_task"] = true

	tests := []struct {
		name    string
		task    types.Task
		wantErr bool
	}{
		{
			name: "dependency met",
			task: types.Task{
				Name:      "Second types.Task",
				Command:   "echo second",
				DependsOn: []string{"first_task"},
			},
			wantErr: false,
		},
		{
			name: "dependency not met",
			task: types.Task{
				Name:      "Third types.Task",
				Command:   "echo third",
				DependsOn: []string{"missing_task"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executor.ExecuteTask(tt.task)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteTask() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExecutor_ExecuteTaskWithVars(t *testing.T) {
	types.ExecOptions.DryRun = true
	defer func() {
		types.ExecOptions.DryRun = false
	}()

	executor := &Executor{
		Host: types.Host{
			Name: "testhost",
		},
		Variables:      make(map[string]interface{}),
		Registers:      make(map[string]string),
		CompletedTasks: make(map[string]bool),
		OutputWriter:   &bytes.Buffer{},
	}

	task := types.Task{
		Name:    "types.Task with vars",
		Command: "echo test",
		Vars: map[string]interface{}{
			"new_var": "new_value",
			"key":     "value",
		},
	}

	err := executor.ExecuteTask(task)
	if err != nil {
		t.Errorf("ExecuteTask() error = %v", err)
	}

	if executor.Variables["new_var"] != "new_value" {
		t.Errorf("Variable 'new_var' = %q, want 'new_value'", executor.Variables["new_var"])
	}
	if executor.Variables["key"] != "value" {
		t.Errorf("Variable 'key' = %q, want 'value'", executor.Variables["key"])
	}
}

func TestExecutor_ExecuteTaskNoType(t *testing.T) {
	types.ExecOptions.DryRun = false
	defer func() {
		types.ExecOptions.DryRun = false
	}()

	executor := &Executor{
		Host: types.Host{
			Name: "testhost",
		},
		Variables:      make(map[string]interface{}),
		Registers:      make(map[string]string),
		CompletedTasks: make(map[string]bool),
		OutputWriter:   &bytes.Buffer{},
	}

	task := types.Task{
		Name: "No Type types.Task",
		// No command, script, copy, or wait_for
	}

	err := executor.ExecuteTask(task)
	if err == nil {
		t.Error("ExecuteTask() should return error for task with no type")
	}
	if !strings.Contains(err.Error(), "no executable task type") {
		t.Errorf("Error should contain 'no executable task type', got: %v", err)
	}
}

func TestExecutor_ExecuteLocalActionDryRun(t *testing.T) {
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
		Name:        "Local Action types.Task",
		LocalAction: "echo 'Running locally'",
	}

	err := executor.ExecuteTask(task)
	if err != nil {
		t.Errorf("ExecuteTask() error = %v", err)
	}

	outputStr := output.String()
	if !strings.Contains(outputStr, "Local Action") {
		t.Errorf("Output should contain 'Local Action', got: %q", outputStr)
	}
}

func TestExecutor_RunOnceAndDelegation(t *testing.T) {
	types.ExecOptions.DryRun = true
	defer func() {
		types.ExecOptions.DryRun = false
		// Reset run_once tracking after the test
		ResetRunOnceTracking()
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

	// Test run_once local_action
	task1 := types.Task{
		Name:        "Run Once Local types.Task",
		LocalAction: "echo 'Running locally once'",
		RunOnce:     true,
	}

	// First execution
	err := executor.ExecuteTask(task1)
	if err != nil {
		t.Errorf("ExecuteTask() error = %v", err)
	}

	outputStr := output.String()
	if !strings.Contains(outputStr, "Local Action") {
		t.Errorf("Output should contain 'Local Action', got: %q", outputStr)
	}
	if !strings.Contains(outputStr, "run once") {
		t.Errorf("Output should contain 'run once', got: %q", outputStr)
	}

	// Reset output buffer
	output.Reset()

	// Second execution should be skipped
	err = executor.ExecuteTask(task1)
	if err != nil {
		t.Errorf("ExecuteTask() error = %v", err)
	}

	outputStr = output.String()
	if !strings.Contains(outputStr, "Skipped") && !strings.Contains(outputStr, "run_once") {
		t.Errorf("Output should indicate task was skipped due to run_once, got: %q", outputStr)
	}

	// Test delegation
	output.Reset()

	task2 := types.Task{
		Name:       "Delegated types.Task",
		Command:    "echo 'Running on delegate'",
		DelegateTo: "otherhost",
	}

	err = executor.ExecuteTask(task2)
	if err != nil {
		t.Errorf("ExecuteTask() error = %v", err)
	}

	outputStr = output.String()
	if !strings.Contains(outputStr, "delegated to") {
		t.Errorf("Output should contain 'delegated to', got: %q", outputStr)
	}
}

func TestExecutor_DelegateTo(t *testing.T) {
	types.ExecOptions.DryRun = true
	defer func() {
		types.ExecOptions.DryRun = false
		ResetRunOnceTracking()
	}()

	// Create two hosts
	host1 := types.Host{Name: "host1", Address: "192.168.1.1"}
	host2 := types.Host{Name: "host2", Address: "192.168.1.2"}

	// Create executors for each host
	var output1, output2 bytes.Buffer

	executor1 := &Executor{
		Host:           host1,
		Variables:      make(map[string]interface{}),
		Registers:      make(map[string]string),
		CompletedTasks: make(map[string]bool),
		OutputWriter:   &output1,
	}

	executor2 := &Executor{
		Host:           host2,
		Variables:      make(map[string]interface{}),
		Registers:      make(map[string]string),
		CompletedTasks: make(map[string]bool),
		OutputWriter:   &output2,
	}

	// Create a task delegated to host2
	delegatedTask := types.Task{
		Name:       "Delegated types.Task",
		Command:    "echo 'Running on delegate'",
		DelegateTo: "host2",
	}

	// Execute on host1 (should be skipped)
	err := executor1.ExecuteTask(delegatedTask)
	if err != nil {
		t.Errorf("ExecuteTask() error = %v", err)
	}

	output1Str := output1.String()
	if !strings.Contains(output1Str, "Skipped") || !strings.Contains(output1Str, "delegated to") {
		t.Errorf("Output should indicate task was skipped due to delegation, got: %q", output1Str)
	}

	// Execute on host2 (should run)
	err = executor2.ExecuteTask(delegatedTask)
	if err != nil {
		t.Errorf("ExecuteTask() error = %v", err)
	}

	output2Str := output2.String()
	if strings.Contains(output2Str, "Skipped") {
		t.Errorf("types.Task should not be skipped on the delegated host, got: %q", output2Str)
	}
	if !strings.Contains(output2Str, "Would execute") {
		t.Errorf("Output should indicate task would execute on the delegated host, got: %q", output2Str)
	}
}

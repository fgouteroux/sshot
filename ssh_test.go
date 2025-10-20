package main

import (
	"os"
	"strings"
	"testing"
)

func TestGetSSHAgent(t *testing.T) {
	// Save original value
	originalAuthSock := os.Getenv("SSH_AUTH_SOCK")
	defer func() {
		if originalAuthSock != "" {
			os.Setenv("SSH_AUTH_SOCK", originalAuthSock)
		} else {
			os.Unsetenv("SSH_AUTH_SOCK")
		}
	}()

	tests := []struct {
		name      string
		authSock  string
		expectNil bool
	}{
		{
			name:      "no auth sock",
			authSock:  "",
			expectNil: true,
		},
		{
			name:      "invalid auth sock",
			authSock:  "/tmp/nonexistent-socket",
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.authSock == "" {
				os.Unsetenv("SSH_AUTH_SOCK")
			} else {
				os.Setenv("SSH_AUTH_SOCK", tt.authSock)
			}

			authMethod := getSSHAgent()

			if tt.expectNil && authMethod != nil {
				t.Error("Expected nil auth method")
			}
			if !tt.expectNil && authMethod == nil {
				t.Error("Expected non-nil auth method")
			}
		})
	}
}

func TestNewExecutor_DryRun(t *testing.T) {
	execOptions.DryRun = true
	execOptions.Verbose = false
	defer func() {
		execOptions.DryRun = false
	}()

	host := Host{
		Name:     "testhost",
		Address:  "127.0.0.1",
		User:     "testuser",
		Password: "testpass",
		Port:     22,
	}

	executor, err := NewExecutor(host, "")
	if err != nil {
		t.Fatalf("NewExecutor failed in dry-run: %v", err)
	}

	if executor == nil {
		t.Fatal("Executor should not be nil")
	}

	if executor.host.Name != host.Name {
		t.Errorf("Executor host name = %q, want %q", executor.host.Name, host.Name)
	}

	if executor.client != nil {
		t.Error("Client should be nil in dry-run mode")
	}

	if len(executor.variables) > 0 {
		t.Errorf("Variables map should be empty, got %d items", len(executor.variables))
	}

	if executor.registers == nil {
		t.Error("Registers map should be initialized")
	}

	if executor.completedTasks == nil {
		t.Error("CompletedTasks map should be initialized")
	}

	err = executor.Close()
	if err != nil {
		t.Errorf("Close() should not error: %v", err)
	}
}

func TestNewExecutor_NoAddress(t *testing.T) {
	execOptions.DryRun = false
	defer func() {
		execOptions.DryRun = false
	}()

	host := Host{
		Name: "testhost",
		User: "testuser",
		// No address or hostname
	}

	_, err := NewExecutor(host, "")
	if err == nil {
		t.Error("NewExecutor should fail with no address")
	}
}

func TestNewExecutor_NoAuthMethod(t *testing.T) {
	execOptions.DryRun = false
	defer func() {
		execOptions.DryRun = false
	}()

	// Clear SSH_AUTH_SOCK to ensure no agent
	originalAuthSock := os.Getenv("SSH_AUTH_SOCK")
	os.Unsetenv("SSH_AUTH_SOCK")
	defer func() {
		if originalAuthSock != "" {
			os.Setenv("SSH_AUTH_SOCK", originalAuthSock)
		}
	}()

	host := Host{
		Name:    "testhost",
		Address: "127.0.0.1",
		User:    "testuser",
		// No password, key file, or agent
	}

	_, err := NewExecutor(host, "")
	if err == nil {
		t.Error("NewExecutor should fail with no auth method")
	}
}

func TestNewExecutor_WithPassword(t *testing.T) {
	execOptions.DryRun = true
	execOptions.Verbose = false
	defer func() {
		execOptions.DryRun = false
	}()

	host := Host{
		Name:     "testhost",
		Address:  "127.0.0.1",
		User:     "testuser",
		Password: "testpass",
	}

	executor, err := NewExecutor(host, "")
	if err != nil {
		t.Fatalf("NewExecutor with password failed: %v", err)
	}

	if executor == nil {
		t.Fatal("Executor should not be nil")
	}
}

func TestNewExecutor_UseAgentEnabled(t *testing.T) {
	execOptions.DryRun = true
	execOptions.Verbose = false
	defer func() {
		execOptions.DryRun = false
	}()

	// Clear the SSH_AUTH_SOCK for this test
	originalAuthSock := os.Getenv("SSH_AUTH_SOCK")
	os.Unsetenv("SSH_AUTH_SOCK")
	defer func() {
		if originalAuthSock != "" {
			os.Setenv("SSH_AUTH_SOCK", originalAuthSock)
		}
	}()

	host := Host{
		Name:     "testhost",
		Address:  "127.0.0.1",
		User:     "testuser",
		UseAgent: true,
	}

	// This should fail because use_agent is true but no agent is available
	_, err := NewExecutor(host, "")
	if err == nil {
		t.Error("NewExecutor should fail when use_agent is true but no agent available")
	}
	if !strings.Contains(err.Error(), "use_agent") {
		t.Errorf("Error should mention use_agent, got: %v", err)
	}
}

func TestNewExecutor_HostnameInsteadOfAddress(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	host := Host{
		Name:     "testhost",
		Hostname: "example.com",
		User:     "testuser",
		Password: "testpass",
	}

	executor, err := NewExecutor(host, "")
	if err != nil {
		t.Fatalf("NewExecutor with hostname failed: %v", err)
	}

	if executor == nil {
		t.Fatal("Executor should not be nil")
	}
}

func TestGetHostKeyCallback_StrictDisabled(t *testing.T) {
	execOptions.Verbose = false

	callback, err := getHostKeyCallback(boolPtr(false))
	if err != nil {
		t.Fatalf("getHostKeyCallback failed: %v", err)
	}

	if callback == nil {
		t.Fatal("Callback should not be nil")
	}
}

func TestGetHostKeyCallback_StrictEnabled(t *testing.T) {
	execOptions.Verbose = false

	callback, err := getHostKeyCallback(boolPtr(true))
	if err != nil {
		t.Fatalf("getHostKeyCallback with strict mode failed: %v", err)
	}

	if callback == nil {
		t.Fatal("Callback should not be nil")
	}
}

func TestExecutor_Close_NilClient(t *testing.T) {
	executor := &Executor{
		client: nil,
	}

	err := executor.Close()
	if err != nil {
		t.Errorf("Close() with nil client should not error: %v", err)
	}
}

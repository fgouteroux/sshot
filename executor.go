package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"golang.org/x/crypto/ssh"
)

func (e *Executor) ExecuteTask(task Task) error {
	writer := e.outputWriter
	if writer == nil {
		writer = os.Stdout
	}

	// Check for delegation - if this task is delegated to a different host,
	// skip it unless we're the delegated host
	if task.DelegateTo != "" && task.DelegateTo != e.host.Name && task.DelegateTo != "localhost" {
		e.mu.Lock()
		fmt.Fprintf(writer, "  ↷ Skipped (delegated to: %s)\n", task.DelegateTo)
		e.completedTasks[task.Name] = true
		e.mu.Unlock()
		return nil
	}

	// If task is delegated to localhost, treat it as a local_action
	if task.DelegateTo == "localhost" && task.Command != "" {
		// Convert to local_action if delegated to localhost
		task.LocalAction = task.Command
		task.Command = ""
	}

	// Check if this is a run_once task that's already been executed
	if task.RunOnce {
		taskKey := task.Name
		runOnceTasks.RLock()
		executed := runOnceTasks.executed[taskKey]
		runOnceTasks.RUnlock()

		if executed {
			e.mu.Lock()
			fmt.Fprintf(writer, "  ↷ Skipped (run_once already executed)\n")
			e.completedTasks[task.Name] = true
			e.mu.Unlock()
			return nil
		}

		// Mark as executed after checking but before actually running
		// to prevent race conditions in parallel execution
		runOnceTasks.Lock()
		runOnceTasks.executed[taskKey] = true
		runOnceTasks.Unlock()
	}

	if execOptions.Verbose {
		e.mu.Lock()
		log.SetOutput(writer)
		log.Printf("[VERBOSE] [%s] Executing task: %s", e.host.Name, task.Name)
		log.SetOutput(os.Stderr)
		e.mu.Unlock()
	}

	if len(task.DependsOn) > 0 {
		for _, dep := range task.DependsOn {
			if !e.completedTasks[dep] {
				return fmt.Errorf("dependency not met: task '%s' depends on '%s' which has not completed", task.Name, dep)
			}
		}
	}

	if task.Vars != nil {
		for k, v := range task.Vars {
			e.variables[k] = v
		}
	}

	if task.When != "" {
		if execOptions.Verbose {
			e.mu.Lock()
			log.SetOutput(writer)
			log.Printf("[VERBOSE] [%s] Evaluating condition: %s", e.host.Name, task.When)
			log.SetOutput(os.Stderr)
			e.mu.Unlock()
		}
		if !e.evaluateCondition(task.When) {
			e.mu.Lock()
			fmt.Fprintf(writer, "  ⊘ Skipped (when: %s)\n", task.When)
			e.completedTasks[task.Name] = true
			e.mu.Unlock()
			return nil
		}
	}

	var output string
	var err error

	if execOptions.DryRun {
		e.mu.Lock()
		fmt.Fprintf(writer, "  🔍 DRY-RUN: Would execute\n")

		// Special handling for delegation
		if task.DelegateTo != "" && task.DelegateTo != e.host.Name && task.DelegateTo != "localhost" {
			fmt.Fprintf(writer, "      Command: %s\n", e.substituteVars(task.Command))
			fmt.Fprintf(writer, "      (would be skipped, delegated to: %s)\n", task.DelegateTo)
			e.completedTasks[task.Name] = true
			e.mu.Unlock()
			return nil
		}

		switch {
		case task.Command != "" && task.DelegateTo != "":
			fmt.Fprintf(writer, "      Command: %s (delegated to: %s)\n",
				e.substituteVars(task.Command), task.DelegateTo)
			if task.RunOnce {
				fmt.Fprintf(writer, "      (run once)\n")
			}
		case task.Command != "":
			fmt.Fprintf(writer, "      Command: %s\n", e.substituteVars(task.Command))
		case task.Shell != "":
			fmt.Fprintf(writer, "      Shell: %s\n", e.substituteVars(task.Shell))
		case task.Script != "":
			fmt.Fprintf(writer, "      Script: %s\n", task.Script)
		case task.LocalAction != "":
			fmt.Fprintf(writer, "      Local Action: %s\n", e.substituteVars(task.LocalAction))
			if task.RunOnce {
				fmt.Fprintf(writer, "      (run once)\n")
			}
		case task.Copy != nil:
			fmt.Fprintf(writer, "      Copy: %s → %s\n", task.Copy.Src, e.substituteVars(task.Copy.Dest))
		}
		if task.Sudo {
			fmt.Fprintf(writer, "      (with sudo)\n")
		}

		if len(task.AllowedExitCodes) > 0 {
			fmt.Fprintf(writer, "      Allowed exit codes: %v\n", task.AllowedExitCodes)
		}

		if len(task.DependsOn) > 0 {
			fmt.Fprintf(writer, "      Dependencies: %v\n", task.DependsOn)
		}
		if task.Retries > 0 {
			fmt.Fprintf(writer, "      Retries: %d (delay: %ds)\n", task.Retries, task.RetryDelay)
		}
		if task.Timeout > 0 {
			fmt.Fprintf(writer, "      Timeout: %ds\n", task.Timeout)
		}
		e.completedTasks[task.Name] = true
		e.mu.Unlock()
		return nil
	}

	// Execute with retry logic
	retries := task.Retries
	if retries == 0 && task.UntilSuccess {
		retries = 60 // Default max retries for until_success
	}
	retryDelay := time.Duration(task.RetryDelay) * time.Second
	if retryDelay == 0 && retries > 0 {
		retryDelay = 5 * time.Second // Default retry delay
	}

	timeout := time.Duration(task.Timeout) * time.Second
	var timeoutTimer *time.Timer
	var timeoutChan <-chan time.Time
	if timeout > 0 {
		timeoutTimer = time.NewTimer(timeout)
		timeoutChan = timeoutTimer.C
		defer timeoutTimer.Stop()
	}

	attempt := 0
	maxAttempts := retries + 1

	for {
		attempt++

		// Execute the task
		switch {
		case task.Command != "":
			output, err = e.executeCommand(task.Command, task.Sudo)
			// Check if the exit code is allowed
			if err != nil && len(task.AllowedExitCodes) > 0 {
				if execOptions.Verbose {
					e.mu.Lock()
					log.SetOutput(writer)
					log.Printf("[VERBOSE] [%s] Command failed with error: %v", e.host.Name, err)
					log.Printf("[VERBOSE] [%s] Checking against allowed exit codes: %v", e.host.Name, task.AllowedExitCodes)
					log.SetOutput(os.Stderr)
					e.mu.Unlock()
				}

				if e.isAllowedExitCode(err, task.AllowedExitCodes) {
					if execOptions.Verbose {
						e.mu.Lock()
						log.SetOutput(writer)
						log.Printf("[VERBOSE] [%s] Exit code is in allowed list, treating as success", e.host.Name)
						log.SetOutput(os.Stderr)
						e.mu.Unlock()
					}
					err = nil
				}
			}
		case task.Shell != "":
			output, err = e.executeCommand(task.Shell, task.Sudo)
			// Check if the exit code is allowed
			if err != nil && len(task.AllowedExitCodes) > 0 {
				if execOptions.Verbose {
					e.mu.Lock()
					log.SetOutput(writer)
					log.Printf("[VERBOSE] [%s] Command failed with error: %v", e.host.Name, err)
					log.Printf("[VERBOSE] [%s] Checking against allowed exit codes: %v", e.host.Name, task.AllowedExitCodes)
					log.SetOutput(os.Stderr)
					e.mu.Unlock()
				}

				if e.isAllowedExitCode(err, task.AllowedExitCodes) {
					if execOptions.Verbose {
						e.mu.Lock()
						log.SetOutput(writer)
						log.Printf("[VERBOSE] [%s] Exit code is in allowed list, treating as success", e.host.Name)
						log.SetOutput(os.Stderr)
						e.mu.Unlock()
					}
					err = nil
				}
			}
		case task.Script != "":
			output, err = e.executeScript(task.Script, task.Sudo)
			// Check if the exit code is allowed
			if err != nil && len(task.AllowedExitCodes) > 0 {
				if execOptions.Verbose {
					e.mu.Lock()
					log.SetOutput(writer)
					log.Printf("[VERBOSE] [%s] Command failed with error: %v", e.host.Name, err)
					log.Printf("[VERBOSE] [%s] Checking against allowed exit codes: %v", e.host.Name, task.AllowedExitCodes)
					log.SetOutput(os.Stderr)
					e.mu.Unlock()
				}

				if e.isAllowedExitCode(err, task.AllowedExitCodes) {
					if execOptions.Verbose {
						e.mu.Lock()
						log.SetOutput(writer)
						log.Printf("[VERBOSE] [%s] Exit code is in allowed list, treating as success", e.host.Name)
						log.SetOutput(os.Stderr)
						e.mu.Unlock()
					}
					err = nil
				}
			}
		case task.LocalAction != "":
			output, err = e.executeLocalAction(task.LocalAction)
			// Check if the exit code is allowed
			if err != nil && len(task.AllowedExitCodes) > 0 {
				if execOptions.Verbose {
					e.mu.Lock()
					log.SetOutput(writer)
					log.Printf("[VERBOSE] [%s] Command failed with error: %v", e.host.Name, err)
					log.Printf("[VERBOSE] [%s] Checking against allowed exit codes: %v", e.host.Name, task.AllowedExitCodes)
					log.SetOutput(os.Stderr)
					e.mu.Unlock()
				}

				if e.isAllowedExitCode(err, task.AllowedExitCodes) {
					if execOptions.Verbose {
						e.mu.Lock()
						log.SetOutput(writer)
						log.Printf("[VERBOSE] [%s] Exit code is in allowed list, treating as success", e.host.Name)
						log.SetOutput(os.Stderr)
						e.mu.Unlock()
					}
					err = nil
				}
			}
		case task.Command != "" && task.DelegateTo != "":
			output, err = e.executeDelegated(task.Command, task.DelegateTo)
			// Check if the exit code is allowed
			if err != nil && len(task.AllowedExitCodes) > 0 {
				if execOptions.Verbose {
					e.mu.Lock()
					log.SetOutput(writer)
					log.Printf("[VERBOSE] [%s] Command failed with error: %v", e.host.Name, err)
					log.Printf("[VERBOSE] [%s] Checking against allowed exit codes: %v", e.host.Name, task.AllowedExitCodes)
					log.SetOutput(os.Stderr)
					e.mu.Unlock()
				}

				if e.isAllowedExitCode(err, task.AllowedExitCodes) {
					if execOptions.Verbose {
						e.mu.Lock()
						log.SetOutput(writer)
						log.Printf("[VERBOSE] [%s] Exit code is in allowed list, treating as success", e.host.Name)
						log.SetOutput(os.Stderr)
						e.mu.Unlock()
					}
					err = nil
				}
			}
		case task.Copy != nil:
			output, err = e.executeCopy(task.Copy)
		case task.WaitFor != "":
			output, err = e.executeWaitFor(task.WaitFor)
		default:
			return fmt.Errorf("no executable task type defined")
		}

		// Success!
		if err == nil {
			if attempt > 1 {
				e.mu.Lock()
				fmt.Fprintf(writer, "  ✓ Success (after %d attempts)\n", attempt)
				e.mu.Unlock()
			}
			break
		}

		// Check timeout
		if timeoutChan != nil {
			select {
			case <-timeoutChan:
				e.mu.Lock()
				fmt.Fprintf(writer, "  ✗ Timeout after %d seconds (attempted %d times)\n", task.Timeout, attempt)
				e.mu.Unlock()
				return fmt.Errorf("timeout after %d seconds: %w", task.Timeout, err)
			default:
			}
		}

		// Check if we should retry
		if attempt >= maxAttempts {
			break
		}

		// Log retry attempt
		if execOptions.Verbose {
			e.mu.Lock()
			log.SetOutput(writer)
			log.Printf("[VERBOSE] [%s] Attempt %d/%d failed, retrying in %v: %v",
				e.host.Name, attempt, maxAttempts, retryDelay, err)
			log.SetOutput(os.Stderr)
			e.mu.Unlock()
		} else {
			e.mu.Lock()
			fmt.Fprintf(writer, "  ⟳ Attempt %d/%d failed, retrying in %v...\n", attempt, maxAttempts, retryDelay)
			e.mu.Unlock()
		}

		// Wait before retry (but check for timeout)
		if timeoutChan != nil {
			select {
			case <-timeoutChan:
				e.mu.Lock()
				fmt.Fprintf(writer, "  ✗ Timeout after %d seconds\n", task.Timeout)
				e.mu.Unlock()
				return fmt.Errorf("timeout after %d seconds: %w", task.Timeout, err)
			case <-time.After(retryDelay):
			}
		} else {
			time.Sleep(retryDelay)
		}
	}

	if task.Register != "" {
		e.registers[task.Register] = output
		e.variables[task.Register] = output
		if execOptions.Verbose {
			e.mu.Lock()
			log.SetOutput(writer)
			log.Printf("[VERBOSE] [%s] Registered output to: %s", e.host.Name, task.Register)
			log.SetOutput(os.Stderr)
			e.mu.Unlock()
		}
	}

	if err != nil {
		if task.IgnoreError {
			e.mu.Lock()
			fmt.Fprintf(writer, "  ⚠ Failed (ignored): %v\n", err)
			if output != "" {
				e.printOutput(writer, output)
			}
			e.completedTasks[task.Name] = true
			e.mu.Unlock()
			return nil
		}
		// Show output on error before returning
		if output != "" {
			e.mu.Lock()
			e.printOutput(writer, output)
			e.mu.Unlock()
		}
		return err
	}

	e.mu.Lock()
	if attempt == 1 {
		fmt.Fprintf(writer, "  %s✓ Success%s\n", color(ColorGreen), color(ColorReset))
	}
	if output != "" {
		e.printOutput(writer, output)
	}
	e.completedTasks[task.Name] = true
	e.mu.Unlock()

	return nil
}

func (e *Executor) executeCommand(cmd string, sudo bool) (string, error) {
	writer := e.outputWriter
	if writer == nil {
		writer = os.Stdout
	}

	cmd = e.substituteVars(cmd)

	if sudo {
		cmd = "sudo -S " + cmd
	}

	if execOptions.Verbose {
		e.mu.Lock()
		log.SetOutput(writer)
		log.Printf("[VERBOSE] [%s] Executing: %s", e.host.Name, cmd)
		log.SetOutput(os.Stderr)
		e.mu.Unlock()
	}

	session, err := e.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Check if we should stream output in real-time
	if execOptions.Progress {
		return e.executeCommandStreaming(session, cmd, writer)
	}

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	err = session.Run(cmd)
	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\nSTDERR: " + stderr.String()
	}

	if execOptions.Verbose {
		e.mu.Lock()
		log.SetOutput(writer)
		log.Printf("[VERBOSE] [%s] Command output length: %d bytes", e.host.Name, len(output))
		log.SetOutput(os.Stderr)
		e.mu.Unlock()
	}

	if err != nil {
		return output, fmt.Errorf("command failed: %w", err)
	}

	return output, nil
}

func (e *Executor) executeCommandStreaming(session *ssh.Session, cmd string, writer io.Writer) (string, error) {
	var outputBuf bytes.Buffer

	// Create pipes for stdout and stderr
	stdout, err := session.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := session.Start(cmd); err != nil {
		return "", fmt.Errorf("failed to start command: %w", err)
	}

	// Stream output in real-time
	var wg sync.WaitGroup
	wg.Add(2)

	// Stream stdout line by line
	go func() {
		defer wg.Done()
		scanner := bytes.NewReader(nil)
		buf := make([]byte, 4096)

		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				data := buf[:n]
				outputBuf.Write(data)

				// Write immediately to output
				e.mu.Lock()
				_, _ = writer.Write([]byte("    │ "))
				_, _ = writer.Write(data)
				if data[n-1] != '\n' {
					_, _ = writer.Write([]byte("\n"))
				}
				e.mu.Unlock()
			}
			if err != nil {
				break
			}
		}
		_ = scanner
	}()

	// Stream stderr line by line
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)

		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				data := buf[:n]
				outputBuf.Write(data)

				// Write immediately to output
				e.mu.Lock()
				_, _ = writer.Write([]byte("    │ [stderr] "))
				_, _ = writer.Write(data)
				if data[n-1] != '\n' {
					_, _ = writer.Write([]byte("\n"))
				}
				e.mu.Unlock()
			}
			if err != nil {
				break
			}
		}
	}()

	// Wait for command to complete
	cmdErr := session.Wait()

	// Wait for all output to be read
	wg.Wait()

	output := outputBuf.String()

	if cmdErr != nil {
		return output, fmt.Errorf("command failed: %w", cmdErr)
	}

	return output, nil
}

func (e *Executor) executeScript(scriptPath string, sudo bool) (string, error) {
	script, err := os.ReadFile(filepath.Clean(scriptPath))
	if err != nil {
		return "", fmt.Errorf("failed to read script: %w", err)
	}

	scriptContent := e.substituteVars(string(script))

	session, err := e.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	tmpFile := fmt.Sprintf("/tmp/script_%d.sh", time.Now().Unix())
	cmd := fmt.Sprintf("cat > %s && chmod +x %s", tmpFile, tmpFile)

	stdin, err := session.StdinPipe()
	if err != nil {
		return "", err
	}

	if err := session.Start(cmd); err != nil {
		return "", err
	}

	_, err = io.WriteString(stdin, scriptContent)
	if err != nil {
		return "", fmt.Errorf("failed to write script content into stdin: %w", err)
	}

	// Close stdin to signal EOF
	if err := stdin.Close(); err != nil {
		return "", fmt.Errorf("failed to close stdin: %w", err)
	}

	if err := session.Wait(); err != nil {
		return "", fmt.Errorf("failed to upload script: %w", err)
	}

	execCmd := tmpFile
	if sudo {
		execCmd = "sudo " + tmpFile
	}

	output, err := e.executeCommand(execCmd, false)
	if err != nil {
		return "", fmt.Errorf("failed to execute script: %w", err)
	}

	_, err = e.executeCommand(fmt.Sprintf("rm -f %s", tmpFile), false)
	if err != nil {
		return "", fmt.Errorf("failed to cleanup script: %w", err)
	}

	return output, err
}

func (e *Executor) executeCopy(copyTask *CopyTask) (string, error) {
	content, err := os.ReadFile(copyTask.Src)
	if err != nil {
		return "", fmt.Errorf("failed to read source file: %w", err)
	}

	contentStr := e.substituteVars(string(content))

	session, err := e.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	dest := e.substituteVars(copyTask.Dest)
	cmd := fmt.Sprintf("cat > %s", dest)

	stdin, err := session.StdinPipe()
	if err != nil {
		return "", err
	}

	if err := session.Start(cmd); err != nil {
		return "", err
	}

	_, err = io.WriteString(stdin, contentStr)
	if err != nil {
		return "", fmt.Errorf("failed to write file content into stdin: %w", err)
	}

	// Close stdin to signal EOF
	if err := stdin.Close(); err != nil {
		return "", fmt.Errorf("failed to close stdin: %w", err)
	}

	if err := session.Wait(); err != nil {
		return "", fmt.Errorf("failed to copy file: %w", err)
	}

	if copyTask.Mode != "" {
		_, err = e.executeCommand(fmt.Sprintf("chmod %s %s", copyTask.Mode, dest), false)
		if err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("Copied %s to %s", copyTask.Src, dest), nil
}

func (e *Executor) executeWaitFor(condition string) (string, error) {
	parts := strings.SplitN(condition, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid wait_for format: %s (expected type:value)", condition)
	}

	waitType := parts[0]
	waitValue := parts[1]

	maxRetries := 30
	retryDelay := 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		var checkCmd string
		switch waitType {
		case "port":
			checkCmd = fmt.Sprintf("nc -z localhost %s", waitValue)
		case "service":
			checkCmd = fmt.Sprintf("systemctl is-active %s", waitValue)
		case "file":
			checkCmd = fmt.Sprintf("test -f %s", waitValue)
		case "http":
			checkCmd = fmt.Sprintf("curl -sf %s", waitValue)
		default:
			return "", fmt.Errorf("unknown wait_for type: %s", waitType)
		}

		_, err := e.executeCommand(checkCmd, false)
		if err == nil {
			return fmt.Sprintf("Condition met: %s", condition), nil
		}

		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	return "", fmt.Errorf("timeout waiting for: %s", condition)
}

func (e *Executor) substituteVars(text string) string {
	tmpl, err := template.New("vars").Parse(text)
	if err != nil {
		return text
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, e.variables); err != nil {
		return text
	}

	return buf.String()
}

func (e *Executor) evaluateCondition(condition string) bool {
	condition = strings.TrimSpace(condition)

	if strings.HasSuffix(condition, "is defined") {
		varName := strings.TrimSuffix(condition, "is defined")
		varName = strings.TrimSpace(varName)
		_, exists := e.variables[varName]
		return exists
	}

	if strings.Contains(condition, "==") {
		parts := strings.Split(condition, "==")
		if len(parts) == 2 {
			left := strings.TrimSpace(e.substituteVars(parts[0]))
			right := strings.TrimSpace(strings.Trim(parts[1], "'\""))
			return left == right
		}
	}

	return true
}

// In executor.go - Update isAllowedExitCode function to focus on command exit codes
func (e *Executor) isAllowedExitCode(err error, allowedCodes []int) bool {
	if err == nil {
		return true
	}

	// If no specific exit codes are allowed, only 0 is acceptable
	if len(allowedCodes) == 0 {
		return false
	}

	// Extract exit code from any error type
	exitCode := extractExitCode(err)
	if exitCode < 0 {
		// No valid exit code found
		return false
	}

	// Check if the exit code is in the allowed list
	for _, allowed := range allowedCodes {
		if exitCode == allowed {
			return true
		}
	}

	return false
}

// Helper function to extract exit code from various error types
func extractExitCode(err error) int {
	// Try SSH ExitError type first
	if exitErr, ok := err.(*ssh.ExitError); ok {
		return exitErr.ExitStatus()
	}

	// Try to parse from error message
	errStr := err.Error()

	// Common patterns for exit codes in error messages
	patterns := []string{
		"Process exited with status ",
		"exit status ",
		"exited with code ",
	}

	for _, pattern := range patterns {
		if idx := strings.Index(errStr, pattern); idx >= 0 {
			codeStr := strings.TrimSpace(errStr[idx+len(pattern):])
			// If there's more text after the number, trim it
			if spaceIdx := strings.Index(codeStr, " "); spaceIdx > 0 {
				codeStr = codeStr[:spaceIdx]
			}

			// Try to parse the exit code
			code, err := strconv.Atoi(codeStr)
			if err == nil {
				return code
			}
		}
	}

	// No valid exit code found
	return -1
}

// printOutput handles output formatting with optional truncation
func (e *Executor) printOutput(writer io.Writer, output string) {
	// If full output is enabled, show everything
	if execOptions.FullOutput {
		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) == 1 {
			fmt.Fprintf(writer, "    %sOutput:%s %s\n", color(ColorGray), color(ColorReset), strings.TrimSpace(output))
		} else {
			fmt.Fprintf(writer, "    %sOutput:%s (%d lines)\n", color(ColorGray), color(ColorReset), len(lines))
			for _, line := range lines {
				fmt.Fprintf(writer, "      %s\n", line)
			}
		}
	} else {
		// Original truncation logic
		if len(output) < 500 {
			fmt.Fprintf(writer, "    %sOutput:%s %s\n", color(ColorGray), color(ColorReset), strings.TrimSpace(output))
		} else {
			lines := strings.Split(strings.TrimSpace(output), "\n")
			if len(lines) <= 10 {
				fmt.Fprintf(writer, "    %sOutput:%s\n", color(ColorGray), color(ColorReset))
				for _, line := range lines {
					fmt.Fprintf(writer, "      %s\n", line)
				}
			} else {
				fmt.Fprintf(writer, "    %sOutput%s (showing first 5 and last 5 lines of %d total):\n", color(ColorGray), color(ColorReset), len(lines))
				for i := 0; i < 5; i++ {
					fmt.Fprintf(writer, "      %s\n", lines[i])
				}
				fmt.Fprintf(writer, "      %s... (%d lines omitted) ...%s\n", color(ColorGray), len(lines)-10, color(ColorReset))
				for i := len(lines) - 5; i < len(lines); i++ {
					fmt.Fprintf(writer, "      %s\n", lines[i])
				}
			}
		}
	}
}

func (e *Executor) executeLocalAction(cmd string) (string, error) {
	cmd = e.substituteVars(cmd)

	if execOptions.Verbose {
		e.mu.Lock()
		log.SetOutput(e.outputWriter)
		log.Printf("[VERBOSE] [%s] Executing locally: %s", e.host.Name, cmd)
		log.SetOutput(os.Stderr)
		e.mu.Unlock()
	}

	// Create command
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	command := exec.Command(parts[0], parts[1:]...)

	// Capture output
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\nSTDERR: " + stderr.String()
	}

	if err != nil {
		return output, fmt.Errorf("local command failed: %w", err)
	}

	return output, nil
}

func (e *Executor) executeDelegated(cmd string, delegateHost string) (string, error) {
	// If delegated to localhost, just run it locally
	if delegateHost == "localhost" || delegateHost == "127.0.0.1" {
		return e.executeLocalAction(cmd)
	}

	// Otherwise, for delegation to happen correctly, the task has to be
	// executed by the proper host's executor. This function should never
	// actually be called since we filter at a higher level.
	return "", fmt.Errorf("delegation to %s should be handled by skipping execution on non-delegate hosts", delegateHost)
}

func ResetRunOnceTracking() {
	runOnceTasks.Lock()
	runOnceTasks.executed = make(map[string]bool)
	runOnceTasks.Unlock()
}

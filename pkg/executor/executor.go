package executor

import (
	"net"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
	"github.com/fgouteroux/sshot/pkg/types"
	"github.com/fgouteroux/sshot/pkg/utils"
	"bytes"
	"encoding/json"
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

type Executor struct {
	Host           types.Host
	client         *ssh.Client
	Variables      map[string]interface{}
	Registers      map[string]string
	CompletedTasks map[string]bool
	GroupName      string
	mu             sync.Mutex
	OutputWriter   io.Writer
	StartTime      time.Time
}

func (e *Executor) CollectFacts(factsConfig types.FactsConfig) error {
	writer := e.OutputWriter
	if writer == nil {
		writer = os.Stdout
	}

	if types.ExecOptions.Verbose {
		fmt.Fprintf(writer, "%s│%s Starting facts collection with %d collectors\n",
			utils.Color(utils.ColorCyan), utils.Color(utils.ColorReset), len(factsConfig.Collectors))
	}

	// Initialize variables map if nil
	if e.Variables == nil {
		e.Variables = make(map[string]interface{})
	}

	for _, collector := range factsConfig.Collectors {
		fmt.Fprintf(writer, "  %s⚙%s Collecting facts: %s\n",
			utils.Color(utils.ColorCyan), utils.Color(utils.ColorReset), collector.Name)

		var output string
		var err error

		if types.ExecOptions.DryRun {
			// In dry-run mode, just show what would be executed
			fmt.Fprintf(writer, "    %s🔍 DRY-RUN:%s Would execute: %s\n",
				utils.Color(utils.ColorYellow), utils.Color(utils.ColorReset), collector.Command)

			// For testing purposes, in dry-run mode, we'll simulate JSON output
			// This allows tests to run without an actual SSH connection
			output = `{"simulated": "data", "dry_run": true}`

			if strings.Contains(collector.Command, "echo") {
				// If the command is an echo command, extract the JSON from it for testing
				jsonStart := strings.Index(collector.Command, "echo '") + 6
				jsonEnd := strings.LastIndex(collector.Command, "'")
				if jsonStart > 6 && jsonEnd > jsonStart {
					output = collector.Command[jsonStart:jsonEnd]
				}
			}
		} else {
			// In real mode, execute the command
			output, err = e.executeCommand(collector.Command, collector.Sudo)
			if err != nil {
				return fmt.Errorf("failed to collect facts with %s: %w", collector.Name, err)
			}
		}

		// Try to parse as JSON
		var factData map[string]interface{}
		if err := json.Unmarshal([]byte(output), &factData); err != nil {
			return fmt.Errorf("failed to parse facts output as JSON: %w", err)
		}

		// Store the facts under the collector name
		e.Variables[collector.Name] = factData

		// Also flatten the structure for easier access in string templates
		flattened := FlattenMap(factData, collector.Name+".")
		for k, v := range flattened {
			e.Variables[k] = v
		}

		if types.ExecOptions.Verbose {
			fmt.Fprintf(writer, "    %s→%s Collected %d fact entries\n",
				utils.Color(utils.ColorGray), utils.Color(utils.ColorReset), len(flattened))
		}
	}

	return nil
}

// Add the missing flattenMap function
func FlattenMap(data map[string]interface{}, prefix string) map[string]string {
	items := make(map[string]string)

	for k, v := range data {
		key := prefix + k

		switch typedVal := v.(type) {
		case map[string]interface{}:
			// Recursively flatten nested maps
			subItems := FlattenMap(typedVal, key+".")
			for sk, sv := range subItems {
				items[sk] = sv
			}

		case []interface{}:
			// Convert arrays to JSON strings
			if jsonBytes, err := json.Marshal(typedVal); err == nil {
				items[key] = string(jsonBytes)
			}

		case string, float64, bool, int:
			// Convert basic types to strings
			items[key] = fmt.Sprintf("%v", typedVal)
		}
	}

	return items
}

func (e *Executor) ExecuteTask(task types.Task) error {
	writer := e.OutputWriter
	if writer == nil {
		writer = os.Stdout
	}

	if types.ExecOptions.Verbose {
		e.mu.Lock()
		log.SetOutput(writer)
		log.Printf("[VERBOSE] [%s] Executing task: %s", e.Host.Name, task.Name)
		log.SetOutput(os.Stderr)
		e.mu.Unlock()
	}

	// Check if task is restricted to specific groups
	if len(task.OnlyGroups) > 0 {
		groupAllowed := false
		for _, group := range task.OnlyGroups {
			if group == e.GroupName {
				groupAllowed = true
				break
			}
		}
		if !groupAllowed {
			// Skip task, not in allowed groups
			fmt.Fprintf(writer, "  ⊘ Skipped (not in allowed groups: %v)\n", task.OnlyGroups)
			return nil
		}
	}

	// Check if task should skip specific groups
	if len(task.SkipGroups) > 0 {
		for _, group := range task.SkipGroups {
			if group == e.GroupName {
				// Skip task, in excluded group
				fmt.Fprintf(writer, "  ⊘ Skipped (in excluded group: %s)\n", group)
				return nil
			}
		}
	}

	// Check for delegation - if this task is delegated to a different host,
	// skip it unless we're the delegated host
	if task.DelegateTo != "" && task.DelegateTo != e.Host.Name && task.DelegateTo != "localhost" {
		e.mu.Lock()
		fmt.Fprintf(writer, "  ↷ Skipped (delegated to: %s)\n", task.DelegateTo)
		e.CompletedTasks[task.Name] = true
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
		types.RunOnceTasks.RLock()
		executed := types.RunOnceTasks.Executed[taskKey]
		types.RunOnceTasks.RUnlock()

		if executed {
			e.mu.Lock()
			fmt.Fprintf(writer, "  ↷ Skipped (run_once already executed)\n")
			e.CompletedTasks[task.Name] = true
			e.mu.Unlock()
			return nil
		}

		// Mark as executed after checking but before actually running
		// to prevent race conditions in parallel execution
		types.RunOnceTasks.Lock()
		types.RunOnceTasks.Executed[taskKey] = true
		types.RunOnceTasks.Unlock()
	}

	if len(task.DependsOn) > 0 {
		for _, dep := range task.DependsOn {
			if !e.CompletedTasks[dep] {
				return fmt.Errorf("dependency not met: task '%s' depends on '%s' which has not completed", task.Name, dep)
			}
		}
	}

	if task.Vars != nil {
		for k, v := range task.Vars {
			e.Variables[k] = v
		}
	}

	if task.When != "" {
		if types.ExecOptions.Verbose {
			e.mu.Lock()
			log.SetOutput(writer)
			log.Printf("[VERBOSE] [%s] Evaluating condition: %s", e.Host.Name, task.When)
			log.SetOutput(os.Stderr)
			e.mu.Unlock()
		}
		if !e.evaluateCondition(task.When) {
			e.mu.Lock()
			fmt.Fprintf(writer, "  ⊘ Skipped (when: %s)\n", task.When)
			e.CompletedTasks[task.Name] = true
			e.mu.Unlock()
			return nil
		}
	}

	var output string
	var err error

	if types.ExecOptions.DryRun {
		e.mu.Lock()
		fmt.Fprintf(writer, "  🔍 DRY-RUN: Would execute\n")

		// Special handling for delegation
		if task.DelegateTo != "" && task.DelegateTo != e.Host.Name && task.DelegateTo != "localhost" {
			fmt.Fprintf(writer, "      Command: %s\n", e.SubstituteVars(task.Command))
			fmt.Fprintf(writer, "      (would be skipped, delegated to: %s)\n", task.DelegateTo)
			e.CompletedTasks[task.Name] = true
			e.mu.Unlock()
			return nil
		}

		switch {
		case task.Command != "" && task.DelegateTo != "":
			fmt.Fprintf(writer, "      Command: %s (delegated to: %s)\n",
				e.SubstituteVars(task.Command), task.DelegateTo)
			if task.RunOnce {
				fmt.Fprintf(writer, "      (run once)\n")
			}
		case task.Command != "":
			fmt.Fprintf(writer, "      Command: %s\n", e.SubstituteVars(task.Command))
		case task.Shell != "":
			fmt.Fprintf(writer, "      Shell: %s\n", e.SubstituteVars(task.Shell))
		case task.Script != "":
			fmt.Fprintf(writer, "      Script: %s\n", task.Script)
		case task.LocalAction != "":
			fmt.Fprintf(writer, "      Local Action: %s\n", e.SubstituteVars(task.LocalAction))
			if task.RunOnce {
				fmt.Fprintf(writer, "      (run once)\n")
			}
		case task.Copy != nil:
			fmt.Fprintf(writer, "      Copy: %s → %s\n", task.Copy.Src, e.SubstituteVars(task.Copy.Dest))
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
		e.CompletedTasks[task.Name] = true
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
				if types.ExecOptions.Verbose {
					e.mu.Lock()
					log.SetOutput(writer)
					log.Printf("[VERBOSE] [%s] Command failed with error: %v", e.Host.Name, err)
					log.Printf("[VERBOSE] [%s] Checking against allowed exit codes: %v", e.Host.Name, task.AllowedExitCodes)
					log.SetOutput(os.Stderr)
					e.mu.Unlock()
				}

				if e.isAllowedExitCode(err, task.AllowedExitCodes) {
					if types.ExecOptions.Verbose {
						e.mu.Lock()
						log.SetOutput(writer)
						log.Printf("[VERBOSE] [%s] Exit code is in allowed list, treating as success", e.Host.Name)
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
				if types.ExecOptions.Verbose {
					e.mu.Lock()
					log.SetOutput(writer)
					log.Printf("[VERBOSE] [%s] Command failed with error: %v", e.Host.Name, err)
					log.Printf("[VERBOSE] [%s] Checking against allowed exit codes: %v", e.Host.Name, task.AllowedExitCodes)
					log.SetOutput(os.Stderr)
					e.mu.Unlock()
				}

				if e.isAllowedExitCode(err, task.AllowedExitCodes) {
					if types.ExecOptions.Verbose {
						e.mu.Lock()
						log.SetOutput(writer)
						log.Printf("[VERBOSE] [%s] Exit code is in allowed list, treating as success", e.Host.Name)
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
				if types.ExecOptions.Verbose {
					e.mu.Lock()
					log.SetOutput(writer)
					log.Printf("[VERBOSE] [%s] Command failed with error: %v", e.Host.Name, err)
					log.Printf("[VERBOSE] [%s] Checking against allowed exit codes: %v", e.Host.Name, task.AllowedExitCodes)
					log.SetOutput(os.Stderr)
					e.mu.Unlock()
				}

				if e.isAllowedExitCode(err, task.AllowedExitCodes) {
					if types.ExecOptions.Verbose {
						e.mu.Lock()
						log.SetOutput(writer)
						log.Printf("[VERBOSE] [%s] Exit code is in allowed list, treating as success", e.Host.Name)
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
				if types.ExecOptions.Verbose {
					e.mu.Lock()
					log.SetOutput(writer)
					log.Printf("[VERBOSE] [%s] Command failed with error: %v", e.Host.Name, err)
					log.Printf("[VERBOSE] [%s] Checking against allowed exit codes: %v", e.Host.Name, task.AllowedExitCodes)
					log.SetOutput(os.Stderr)
					e.mu.Unlock()
				}

				if e.isAllowedExitCode(err, task.AllowedExitCodes) {
					if types.ExecOptions.Verbose {
						e.mu.Lock()
						log.SetOutput(writer)
						log.Printf("[VERBOSE] [%s] Exit code is in allowed list, treating as success", e.Host.Name)
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
				if types.ExecOptions.Verbose {
					e.mu.Lock()
					log.SetOutput(writer)
					log.Printf("[VERBOSE] [%s] Command failed with error: %v", e.Host.Name, err)
					log.Printf("[VERBOSE] [%s] Checking against allowed exit codes: %v", e.Host.Name, task.AllowedExitCodes)
					log.SetOutput(os.Stderr)
					e.mu.Unlock()
				}

				if e.isAllowedExitCode(err, task.AllowedExitCodes) {
					if types.ExecOptions.Verbose {
						e.mu.Lock()
						log.SetOutput(writer)
						log.Printf("[VERBOSE] [%s] Exit code is in allowed list, treating as success", e.Host.Name)
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
		if types.ExecOptions.Verbose {
			e.mu.Lock()
			log.SetOutput(writer)
			log.Printf("[VERBOSE] [%s] Attempt %d/%d failed, retrying in %v: %v",
				e.Host.Name, attempt, maxAttempts, retryDelay, err)
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
		e.Registers[task.Register] = output
		e.Variables[task.Register] = output
		if types.ExecOptions.Verbose {
			e.mu.Lock()
			log.SetOutput(writer)
			log.Printf("[VERBOSE] [%s] Registered output to: %s", e.Host.Name, task.Register)
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
			e.CompletedTasks[task.Name] = true
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
		fmt.Fprintf(writer, "  %s✓ Success%s\n", utils.Color(utils.ColorGreen), utils.Color(utils.ColorReset))
	}
	if output != "" {
		e.printOutput(writer, output)
	}
	e.CompletedTasks[task.Name] = true
	e.mu.Unlock()

	return nil
}

func (e *Executor) executeCommand(cmd string, sudo bool) (string, error) {
	writer := e.OutputWriter
	if writer == nil {
		writer = os.Stdout
	}

	cmd = e.SubstituteVars(cmd)

	if sudo {
		cmd = "sudo -S " + cmd
	}

	if types.ExecOptions.Verbose {
		e.mu.Lock()
		log.SetOutput(writer)
		log.Printf("[VERBOSE] [%s] Executing: %s", e.Host.Name, cmd)
		log.SetOutput(os.Stderr)
		e.mu.Unlock()
	}

	// Handle dry-run mode
	if types.ExecOptions.DryRun {
		e.mu.Lock()
		fmt.Fprintf(writer, "  🔍 DRY-RUN: Would execute: %s\n", cmd)
		e.mu.Unlock()

		// For testing purposes in dry-run mode, we can simulate output
		// Check if this is an echo command and extract content
		if strings.HasPrefix(cmd, "echo ") {
			content := cmd[5:] // Skip "echo "
			// Handle various quoting styles
			if (strings.HasPrefix(content, "'") && strings.HasSuffix(content, "'")) ||
				(strings.HasPrefix(content, "\"") && strings.HasSuffix(content, "\"")) {
				return content[1 : len(content)-1], nil
			}
			return content, nil
		}

		return "DRY-RUN: Command would execute", nil
	}

	session, err := e.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Check if we should stream output in real-time
	if types.ExecOptions.Progress {
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

	if types.ExecOptions.Verbose {
		e.mu.Lock()
		log.SetOutput(writer)
		log.Printf("[VERBOSE] [%s] Command output length: %d bytes", e.Host.Name, len(output))
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

	scriptContent := e.SubstituteVars(string(script))

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

func (e *Executor) executeCopy(copyTask *types.CopyTask) (string, error) {
	content, err := os.ReadFile(copyTask.Src)
	if err != nil {
		return "", fmt.Errorf("failed to read source file: %w", err)
	}

	contentStr := e.SubstituteVars(string(content))

	session, err := e.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	dest := e.SubstituteVars(copyTask.Dest)
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

func (e *Executor) SubstituteVars(text string) string {
	// Create a template with helper functions
	funcMap := template.FuncMap{
		"fact": func(path string) string {
			// Allow accessing facts with dot notation: {{ fact "puppet_facts.os.family" }}
			parts := strings.Split(path, ".")
			if len(parts) < 1 {
				return ""
			}

			// Navigate through the nested structure
			current, exists := e.Variables[parts[0]]
			if !exists {
				return ""
			}

			for i := 1; i < len(parts); i++ {
				if m, ok := current.(map[string]interface{}); ok {
					var exists bool
					current, exists = m[parts[i]]
					if !exists {
						return ""
					}
				} else {
					return ""
				}
			}

			return fmt.Sprintf("%v", current)
		},
	}

	tmpl, err := template.New("vars").Funcs(funcMap).Parse(text)
	if err != nil {
		return text
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, e.Variables); err != nil {
		return text
	}

	return buf.String()
}

func (e *Executor) evaluateCondition(condition string) bool {
	condition = strings.TrimSpace(condition)

	if strings.HasSuffix(condition, "is defined") {
		varName := strings.TrimSuffix(condition, "is defined")
		varName = strings.TrimSpace(varName)
		_, exists := e.Variables[varName]
		return exists
	}

	if strings.Contains(condition, "==") {
		parts := strings.Split(condition, "==")
		if len(parts) == 2 {
			left := strings.TrimSpace(e.SubstituteVars(parts[0]))
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
	if types.ExecOptions.FullOutput {
		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) == 1 {
			fmt.Fprintf(writer, "    %sOutput:%s %s\n", utils.Color(utils.ColorGray), utils.Color(utils.ColorReset), strings.TrimSpace(output))
		} else {
			fmt.Fprintf(writer, "    %sOutput:%s (%d lines)\n", utils.Color(utils.ColorGray), utils.Color(utils.ColorReset), len(lines))
			for _, line := range lines {
				fmt.Fprintf(writer, "      %s\n", line)
			}
		}
	} else {
		// Original truncation logic
		if len(output) < 500 {
			fmt.Fprintf(writer, "    %sOutput:%s %s\n", utils.Color(utils.ColorGray), utils.Color(utils.ColorReset), strings.TrimSpace(output))
		} else {
			lines := strings.Split(strings.TrimSpace(output), "\n")
			if len(lines) <= 10 {
				fmt.Fprintf(writer, "    %sOutput:%s\n", utils.Color(utils.ColorGray), utils.Color(utils.ColorReset))
				for _, line := range lines {
					fmt.Fprintf(writer, "      %s\n", line)
				}
			} else {
				fmt.Fprintf(writer, "    %sOutput%s (showing first 5 and last 5 lines of %d total):\n", utils.Color(utils.ColorGray), utils.Color(utils.ColorReset), len(lines))
				for i := 0; i < 5; i++ {
					fmt.Fprintf(writer, "      %s\n", lines[i])
				}
				fmt.Fprintf(writer, "      %s... (%d lines omitted) ...%s\n", utils.Color(utils.ColorGray), len(lines)-10, utils.Color(utils.ColorReset))
				for i := len(lines) - 5; i < len(lines); i++ {
					fmt.Fprintf(writer, "      %s\n", lines[i])
				}
			}
		}
	}
}

func (e *Executor) executeLocalAction(cmd string) (string, error) {
	cmd = e.SubstituteVars(cmd)

	if types.ExecOptions.Verbose {
		e.mu.Lock()
		log.SetOutput(e.OutputWriter)
		log.Printf("[VERBOSE] [%s] Executing locally: %s", e.Host.Name, cmd)
		log.SetOutput(os.Stderr)
		e.mu.Unlock()
	}

	// Create command
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	command := exec.Command("/bin/sh", "-c", cmd)

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
	types.RunOnceTasks.Lock()
	types.RunOnceTasks.Executed = make(map[string]bool)
	types.RunOnceTasks.Unlock()
}

func NewExecutor(host types.Host, groupName string) (*Executor, error) {
	if types.ExecOptions.Verbose {
		log.Printf("[VERBOSE] Connecting to Host: %s", host.Name)
	}

	// Get host key callback for verification
	hostKeyCallback, err := getHostKeyCallback(host.StrictHostKeyCheck)
	if err != nil {
		return nil, fmt.Errorf("failed to load host keys: %w", err)
	}

	config := &ssh.ClientConfig{
		User:            host.User,
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}

	var authMethods []ssh.AuthMethod

	// Determine which UseAgent to use (host-specific or default)
	useAgent := host.UseAgent

	if useAgent || (host.KeyFile == "" && host.Password == "" && os.Getenv("SSH_AUTH_SOCK") != "") {
		if agentAuth := getSSHAgent(); agentAuth != nil {
			authMethods = append(authMethods, agentAuth)
			if types.ExecOptions.Verbose {
				log.Printf("[VERBOSE] [%s] Using ssh-agent for authentication", host.Name)
			}
		} else if useAgent {
			return nil, fmt.Errorf("use_agent is true but ssh-agent is not available")
		}
	}

	if host.KeyFile != "" {
		keyPath := host.KeyFile
		if strings.HasPrefix(keyPath, "~/") {
			homeDir, err := os.UserHomeDir()
			if err == nil {
				keyPath = strings.Replace(keyPath, "~", homeDir, 1)
			}
		}

		if types.ExecOptions.Verbose {
			log.Printf("[VERBOSE] [%s] Reading key file: %s", host.Name, keyPath)
		}

		key, err := os.ReadFile(filepath.Clean(keyPath))
		if err != nil {
			return nil, fmt.Errorf("unable to read private key: %w", err)
		}

		var signer ssh.Signer
		if host.KeyPassword != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(host.KeyPassword))
			if err != nil {
				return nil, fmt.Errorf("unable to parse private key with passphrase: %w", err)
			}
		} else {
			signer, err = ssh.ParsePrivateKey(key)
			if err != nil {
				fmt.Printf("Private key for %s appears to be passphrase protected.\n", host.Name)
				fmt.Printf("Enter passphrase for %s: ", host.KeyFile)
				var passphrase string
				_, err = fmt.Scanln(&passphrase)
				if err != nil {
					return nil, fmt.Errorf("unable to read stdin for private key passphrase: %w", err)
				}
				signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(passphrase))
				if err != nil {
					return nil, fmt.Errorf("unable to parse private key with passphrase: %w", err)
				}
			}
		}

		authMethods = append(authMethods, ssh.PublicKeys(signer))
		if types.ExecOptions.Verbose {
			log.Printf("[VERBOSE] [%s] Using key file: %s", host.Name, keyPath)
		}
	}

	if host.Password != "" {
		authMethods = append(authMethods, ssh.Password(host.Password))
		if types.ExecOptions.Verbose {
			log.Printf("[VERBOSE] [%s] Using password authentication", host.Name)
		}
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication method provided (try: use_agent: true, key_file, or password)")
	}

	config.Auth = authMethods

	port := host.Port
	if port == 0 {
		port = 22
	}

	target := host.Address
	if target == "" {
		target = host.Hostname
	}
	if target == "" {
		return nil, fmt.Errorf("no address or hostname provided")
	}

	if types.ExecOptions.Verbose {
		log.Printf("[VERBOSE] [%s] Dialing %s:%d", host.Name, target, port)
	}

	if types.ExecOptions.DryRun {
		if types.ExecOptions.Verbose {
			log.Printf("[VERBOSE] [%s] DRY-RUN: Skipping actual SSH connection", host.Name)
		}
		vars := make(map[string]interface{})
		if host.Vars != nil {
			for k, v := range host.Vars {
				vars[k] = v
			}
		}

		return &Executor{
			Host:           host,
			client:         nil,
			Variables:      vars,
			Registers:      make(map[string]string),
			CompletedTasks: make(map[string]bool),
			GroupName:      groupName,
			OutputWriter:   os.Stdout,
			StartTime:      time.Now(),
		}, nil
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", target, port), config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	if types.ExecOptions.Verbose {
		log.Printf("[VERBOSE] [%s] Successfully connected", host.Name)
	}

	return &Executor{
		Host:           host,
		client:         client,
		Variables:      host.Vars,
		Registers:      make(map[string]string),
		CompletedTasks: make(map[string]bool),
		GroupName:      groupName,
		OutputWriter:   os.Stdout,
		StartTime:      time.Now(),
	}, nil
}

func getSSHAgent() ssh.AuthMethod {
	sshAuthSock := os.Getenv("SSH_AUTH_SOCK")
	if sshAuthSock == "" {
		return nil
	}

	conn, err := net.Dial("unix", sshAuthSock)
	if err != nil {
		return nil
	}

	agentClient := agent.NewClient(conn)
	return ssh.PublicKeysCallback(agentClient.Signers)
}

func (e *Executor) Close() error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}

func getHostKeyCallback(strictHostKeyCheck *bool) (ssh.HostKeyCallback, error) {
	// Determine the actual value to use (default to true if nil)
	strict := true
	if strictHostKeyCheck != nil {
		strict = *strictHostKeyCheck
	}

	// If strict host key checking is disabled, use insecure callback
	// This is useful for testing environments but should be avoided in production
	if !strict {
		if types.ExecOptions.Verbose {
			log.Printf("[VERBOSE] WARNING: types.Host key verification is disabled (strict_host_key_check: false)")
		}
		return ssh.InsecureIgnoreHostKey(), nil //gosec:disable G106
	}

	// Try to load known_hosts file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("unable to get home directory: %w", err)
	}

	knownHostsPath := filepath.Join(homeDir, ".ssh", "known_hosts")

	// Check if known_hosts exists
	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		// Create .ssh directory if it doesn't exist
		sshDir := filepath.Join(homeDir, ".ssh")
		if err := os.MkdirAll(sshDir, 0700); err != nil {
			return nil, fmt.Errorf("unable to create .ssh directory: %w", err)
		}

		// Create empty known_hosts file
		if _, err := os.Create(filepath.Clean(knownHostsPath)); err != nil {
			return nil, fmt.Errorf("unable to create known_hosts file: %w", err)
		}

		if types.ExecOptions.Verbose {
			log.Printf("[VERBOSE] Created new known_hosts file at: %s", knownHostsPath)
		}
	}

	// Load known_hosts
	hostKeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("unable to load known_hosts: %w", err)
	}

	// Wrap the callback to provide better error messages
	return ssh.HostKeyCallback(func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := hostKeyCallback(hostname, remote, key)
		if err != nil {
			// Extract hostname without port for ssh-keyscan command
			host, _, splitErr := net.SplitHostPort(hostname)
			if splitErr != nil {
				// If splitting fails, use the original hostname
				host = hostname
			}

			// Check if this is a host key mismatch or unknown host
			if keyErr, ok := err.(*knownhosts.KeyError); ok && len(keyErr.Want) > 0 {
				return fmt.Errorf("host key verification failed for %s: %w\nThe host key has changed. This could indicate a security breach.\nIf you trust this host, remove the old key from %s", hostname, err, knownHostsPath)
			}
			return fmt.Errorf("host key verification failed for %s: %w\nTo add this host, run: ssh-keyscan -H %s >> %s", hostname, err, host, knownHostsPath)
		}
		return nil
	}), nil
}

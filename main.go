package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"text/template"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Inventory Inventory `yaml:"inventory"`
	Playbook  Playbook  `yaml:"playbook"`
}

type ExecutionOptions struct {
	DryRun   bool
	Verbose  bool
	Parallel bool
	Progress bool
	NoColor  bool
}

var execOptions ExecutionOptions

// ANSI color codes
const (
	ColorReset   = "\033[0m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorGray    = "\033[90m"
	ColorBold    = "\033[1m"
)

func colorize(color, text string) string {
	if execOptions.NoColor {
		return text
	}
	return color + text + ColorReset
}

type Inventory struct {
	Hosts     []Host      `yaml:"hosts,omitempty"`
	Groups    []Group     `yaml:"groups,omitempty"`
	SSHConfig *SSHConfig  `yaml:"ssh_config,omitempty"`
}

type SSHConfig struct {
	User        string `yaml:"user,omitempty"`
	Password    string `yaml:"password,omitempty"`
	KeyFile     string `yaml:"key_file,omitempty"`
	KeyPassword string `yaml:"key_password,omitempty"`
	UseAgent    bool   `yaml:"use_agent,omitempty"`
	Port        int    `yaml:"port,omitempty"`
}

type Group struct {
	Name      string   `yaml:"name"`
	Hosts     []Host   `yaml:"hosts"`
	Parallel  bool     `yaml:"parallel,omitempty"`
	Order     int      `yaml:"order,omitempty"`
	DependsOn []string `yaml:"depends_on,omitempty"`
}

type Host struct {
	Name        string            `yaml:"name"`
	Address     string            `yaml:"address,omitempty"`
	Hostname    string            `yaml:"hostname,omitempty"`
	Port        int               `yaml:"port"`
	User        string            `yaml:"user"`
	Password    string            `yaml:"password,omitempty"`
	KeyFile     string            `yaml:"key_file,omitempty"`
	KeyPassword string            `yaml:"key_password,omitempty"`
	UseAgent    bool              `yaml:"use_agent,omitempty"`
	Vars        map[string]string `yaml:"vars,omitempty"`
}

type Playbook struct {
	Name     string `yaml:"name"`
	Parallel bool   `yaml:"parallel,omitempty"`
	Tasks    []Task `yaml:"tasks"`
}

type Task struct {
	Name        string            `yaml:"name"`
	Command     string            `yaml:"command,omitempty"`
	Script      string            `yaml:"script,omitempty"`
	Copy        *CopyTask         `yaml:"copy,omitempty"`
	Shell       string            `yaml:"shell,omitempty"`
	Sudo        bool              `yaml:"sudo,omitempty"`
	When        string            `yaml:"when,omitempty"`
	Register    string            `yaml:"register,omitempty"`
	IgnoreError bool              `yaml:"ignore_error,omitempty"`
	Vars        map[string]string `yaml:"vars,omitempty"`
	DependsOn   []string          `yaml:"depends_on,omitempty"`
	WaitFor     string            `yaml:"wait_for,omitempty"`
	Retries     int               `yaml:"retries,omitempty"`
	RetryDelay  int               `yaml:"retry_delay,omitempty"`
	Timeout     int               `yaml:"timeout,omitempty"`
	UntilSuccess bool             `yaml:"until_success,omitempty"`
	Stream      bool              `yaml:"stream,omitempty"`
}

type CopyTask struct {
	Src  string `yaml:"src"`
	Dest string `yaml:"dest"`
	Mode string `yaml:"mode,omitempty"`
}

type Executor struct {
	host           Host
	client         *ssh.Client
	variables      map[string]string
	registers      map[string]string
	completedTasks map[string]bool
	mu             sync.Mutex
	outputWriter   io.Writer
	startTime      time.Time
}

func NewExecutor(host Host) (*Executor, error) {
	if execOptions.Verbose {
		log.Printf("[VERBOSE] Connecting to host: %s", host.Name)
	}

	config := &ssh.ClientConfig{
		User:            host.User,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	var authMethods []ssh.AuthMethod

	// Determine which UseAgent to use (host-specific or default)
	useAgent := host.UseAgent
	
	if useAgent || (host.KeyFile == "" && host.Password == "" && os.Getenv("SSH_AUTH_SOCK") != "") {
		if agentAuth := getSSHAgent(); agentAuth != nil {
			authMethods = append(authMethods, agentAuth)
			if execOptions.Verbose {
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

		if execOptions.Verbose {
			log.Printf("[VERBOSE] [%s] Reading key file: %s", host.Name, keyPath)
		}

		key, err := os.ReadFile(keyPath)
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
				fmt.Scanln(&passphrase)
				signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(passphrase))
				if err != nil {
					return nil, fmt.Errorf("unable to parse private key with passphrase: %w", err)
				}
			}
		}

		authMethods = append(authMethods, ssh.PublicKeys(signer))
		if execOptions.Verbose {
			log.Printf("[VERBOSE] [%s] Using key file: %s", host.Name, keyPath)
		}
	}

	if host.Password != "" {
		authMethods = append(authMethods, ssh.Password(host.Password))
		if execOptions.Verbose {
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

	if execOptions.Verbose {
		log.Printf("[VERBOSE] [%s] Dialing %s:%d", host.Name, target, port)
	}

	if execOptions.DryRun {
		if execOptions.Verbose {
			log.Printf("[VERBOSE] [%s] DRY-RUN: Skipping actual SSH connection", host.Name)
		}
		return &Executor{
			host:           host,
			client:         nil,
			variables:      host.Vars,
			registers:      make(map[string]string),
			completedTasks: make(map[string]bool),
			outputWriter:   os.Stdout,
			startTime:      time.Now(),
		}, nil
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", target, port), config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %w", err)
	}

	if execOptions.Verbose {
		log.Printf("[VERBOSE] [%s] Successfully connected", host.Name)
	}

	return &Executor{
		host:           host,
		client:         client,
		variables:      host.Vars,
		registers:      make(map[string]string),
		completedTasks: make(map[string]bool),
		outputWriter:   os.Stdout,
		startTime:      time.Now(),
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

func (e *Executor) ExecuteTask(task Task) error {
	writer := e.outputWriter
	if writer == nil {
		writer = os.Stdout
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
		switch {
		case task.Command != "":
			fmt.Fprintf(writer, "      Command: %s\n", e.substituteVars(task.Command))
		case task.Shell != "":
			fmt.Fprintf(writer, "      Shell: %s\n", e.substituteVars(task.Shell))
		case task.Script != "":
			fmt.Fprintf(writer, "      Script: %s\n", task.Script)
		case task.Copy != nil:
			fmt.Fprintf(writer, "      Copy: %s → %s\n", task.Copy.Src, e.substituteVars(task.Copy.Dest))
		}
		if task.Sudo {
			fmt.Fprintf(writer, "      (with sudo)\n")
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
		case task.Shell != "":
			output, err = e.executeCommand(task.Shell, task.Sudo)
		case task.Script != "":
			output, err = e.executeScript(task.Script, task.Sudo)
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
				fmt.Fprintf(writer, "    Output: %s\n", strings.TrimSpace(output))
			}
			e.completedTasks[task.Name] = true
			e.mu.Unlock()
			return nil
		}
		return err
	}

	e.mu.Lock()
	if attempt == 1 {
		fmt.Fprintf(writer, "  %s✓ Success%s\n", ColorGreen, ColorReset)
	}
	if output != "" {
		if len(output) < 500 {
			fmt.Fprintf(writer, "    %sOutput:%s %s\n", ColorGray, ColorReset, strings.TrimSpace(output))
		} else {
			lines := strings.Split(strings.TrimSpace(output), "\n")
			if len(lines) <= 10 {
				fmt.Fprintf(writer, "    %sOutput:%s\n", ColorGray, ColorReset)
				for _, line := range lines {
					fmt.Fprintf(writer, "      %s\n", line)
				}
			} else {
				fmt.Fprintf(writer, "    %sOutput%s (showing first 5 and last 5 lines of %d total):\n", ColorGray, ColorReset, len(lines))
				for i := 0; i < 5; i++ {
					fmt.Fprintf(writer, "      %s\n", lines[i])
				}
				fmt.Fprintf(writer, "      %s... (%d lines omitted) ...%s\n", ColorGray, len(lines)-10, ColorReset)
				for i := len(lines) - 5; i < len(lines); i++ {
					fmt.Fprintf(writer, "      %s\n", lines[i])
				}
			}
		}
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
				writer.Write([]byte("    │ "))
				writer.Write(data)
				if data[n-1] != '\n' {
					writer.Write([]byte("\n"))
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
				writer.Write([]byte("    │ [stderr] "))
				writer.Write(data)
				if data[n-1] != '\n' {
					writer.Write([]byte("\n"))
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
	script, err := os.ReadFile(scriptPath)
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

	io.WriteString(stdin, scriptContent)
	stdin.Close()

	if err := session.Wait(); err != nil {
		return "", fmt.Errorf("failed to upload script: %w", err)
	}

	execCmd := tmpFile
	if sudo {
		execCmd = "sudo " + tmpFile
	}

	output, err := e.executeCommand(execCmd, false)

	e.executeCommand(fmt.Sprintf("rm -f %s", tmpFile), false)

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

	io.WriteString(stdin, contentStr)
	stdin.Close()

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

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	} else if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

type HostResult struct {
	Host    Host
	Success bool
	Error   error
	Output  string
}

func executeOnHost(host Host, tasks []Task, captureOutput bool) HostResult {
	var output bytes.Buffer
	var writer io.Writer = os.Stdout

	if captureOutput {
		writer = &output
	}

	displayTarget := host.Address
	if displayTarget == "" {
		displayTarget = host.Hostname
	}

	hostStart := time.Now()

	fmt.Fprintf(writer, "%s┌─ Host: %s%s%s (%s)\n", ColorCyan, ColorBold, host.Name, ColorReset, displayTarget)
	fmt.Fprintf(writer, "%s│%s\n", ColorCyan, ColorReset)

	executor, err := NewExecutor(host)
	if err != nil {
		fmt.Fprintf(writer, "%s│%s %s✗ Connection failed:%s %v\n", ColorCyan, ColorReset, ColorRed, ColorReset, err)
		fmt.Fprintf(writer, "%s└─ ✗ Connection Failed%s\n\n", ColorRed, ColorReset)
		return HostResult{Host: host, Success: false, Error: err, Output: output.String()}
	}
	defer executor.Close()

	if captureOutput {
		executor.outputWriter = writer
	}

	for i, task := range tasks {
		taskStart := time.Now()
		fmt.Fprintf(writer, "%s│%s [%d/%d] %s\n", ColorCyan, ColorReset, i+1, len(tasks), task.Name)

		if err := executor.ExecuteTask(task); err != nil {
			taskDuration := time.Since(taskStart)
			log.SetOutput(writer)
			log.Printf("  %s✗%s Task failed after %s: %v\n", ColorRed, ColorReset, formatDuration(taskDuration), err)
			log.SetOutput(os.Stderr)
			fmt.Fprintf(writer, "%s└─ ✗ Failed%s (total time: %s)\n\n", ColorRed, ColorReset, formatDuration(time.Since(hostStart)))
			return HostResult{Host: host, Success: false, Error: err, Output: output.String()}
		}

		taskDuration := time.Since(taskStart)
		if execOptions.Verbose || taskDuration > 1*time.Second {
			fmt.Fprintf(writer, "%s│%s         %s⏱%s  Task took %s%s%s\n", 
				ColorCyan, ColorReset, ColorGray, ColorReset, ColorCyan, formatDuration(taskDuration), ColorReset)
		}
	}

	totalDuration := time.Since(hostStart)
	fmt.Fprintf(writer, "%s└─ ✓ Completed%s (total time: %s%s%s)\n\n", 
		ColorGreen, ColorReset, ColorCyan, formatDuration(totalDuration), ColorReset)
	return HostResult{Host: host, Success: true, Error: nil, Output: output.String()}
}

func executeHostsParallel(hosts []Host, tasks []Task) []HostResult {
	var wg sync.WaitGroup
	resultsChan := make(chan HostResult, len(hosts))

	for _, host := range hosts {
		wg.Add(1)
		go func(h Host) {
			defer wg.Done()
			result := executeOnHost(h, tasks, true)
			resultsChan <- result
		}(host)
	}

	wg.Wait()
	close(resultsChan)

	var results []HostResult
	for result := range resultsChan {
		results = append(results, result)
	}

	for _, host := range hosts {
		for _, result := range results {
			if result.Host.Name == host.Name {
				fmt.Print(result.Output)
				break
			}
		}
	}

	return results
}

func executeHostsSequential(hosts []Host, tasks []Task) []HostResult {
	var results []HostResult

	for _, host := range hosts {
		result := executeOnHost(host, tasks, false)
		results = append(results, result)

		if !result.Success {
			break
		}
	}

	return results
}

func executeWithGroups(config Config) ([]HostResult, error) {
	groups := config.Inventory.Groups

	sortedGroups := make([]Group, len(groups))
	copy(sortedGroups, groups)

	for i := 0; i < len(sortedGroups); i++ {
		for j := i + 1; j < len(sortedGroups); j++ {
			if sortedGroups[j].Order < sortedGroups[i].Order {
				sortedGroups[i], sortedGroups[j] = sortedGroups[j], sortedGroups[i]
			}
		}
	}

	if execOptions.Verbose {
		log.Printf("[VERBOSE] Executing %d groups in order", len(sortedGroups))
		for _, g := range sortedGroups {
			log.Printf("[VERBOSE]   Group: %s (order: %d, hosts: %d, parallel: %v)",
				g.Name, g.Order, len(g.Hosts), g.Parallel)
		}
	}

	var allResults []HostResult
	completedGroups := make(map[string]bool)

	for _, group := range sortedGroups {
		if len(group.DependsOn) > 0 {
			for _, dep := range group.DependsOn {
				if !completedGroups[dep] {
					return allResults, fmt.Errorf("group '%s' depends on '%s' which has not completed", group.Name, dep)
				}
			}
		}

		fmt.Printf("\n%s═══ Group: %s%s%s (order: %d) ═══%s\n", ColorMagenta, ColorBold, group.Name, ColorReset, group.Order, ColorReset)
		if len(group.DependsOn) > 0 {
			fmt.Printf("    %sDependencies:%s %v\n", ColorReset, ColorReset, group.DependsOn)
		}
		fmt.Printf("\n")

		var groupResults []HostResult

		if group.Parallel || execOptions.Parallel {
			groupResults = executeHostsParallel(group.Hosts, config.Playbook.Tasks)
		} else {
			groupResults = executeHostsSequential(group.Hosts, config.Playbook.Tasks)
		}

		allResults = append(allResults, groupResults...)

		groupFailed := false
		for _, result := range groupResults {
			if !result.Success {
				groupFailed = true
			}
		}

		if groupFailed {
			return allResults, fmt.Errorf("group '%s' failed", group.Name)
		}

		completedGroups[group.Name] = true
	}

	return allResults, nil
}

func printPlaybookSummary(results []HostResult, totalDuration time.Duration, err error) {
	successCount := 0
	failCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			failCount++
		}
	}

	fmt.Printf("╔════════════════════════════════════════════════════════════════╗\n")
	if err != nil || failCount > 0 {
		fmt.Printf("║  ✗ PLAYBOOK FAILED                                             ║\n")
		fmt.Printf("║    Successful: %-3d  Failed: %-3d                                ║\n", successCount, failCount)
		fmt.Printf("║    Total time: %-47s ║\n", formatDuration(totalDuration))
		fmt.Printf("╚════════════════════════════════════════════════════════════════╝\n\n")

		if execOptions.Verbose {
			if err != nil {
				log.Printf("[VERBOSE] Error: %v", err)
			}
			log.Printf("[VERBOSE] Failed hosts:")
			for _, result := range results {
				if !result.Success {
					log.Printf("[VERBOSE]   - %s: %v", result.Host.Name, result.Error)
				}
			}
		}
	} else {
		if execOptions.DryRun {
			fmt.Printf("║  ✓ DRY-RUN COMPLETED                                           ║\n")
		} else {
			fmt.Printf("║  ✓ PLAYBOOK COMPLETED SUCCESSFULLY                             ║\n")
		}
		fmt.Printf("║    All %d host(s) completed successfully                        ║\n", successCount)
		fmt.Printf("║    Total time: %-47s ║\n", formatDuration(totalDuration))
		fmt.Printf("╚════════════════════════════════════════════════════════════════╝\n\n")
	}
}

func RunPlaybook(configPath string) error {
	playbookStart := time.Now()
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Apply SSH defaults to hosts
	applySSHDefaults(&config)

	parallel := execOptions.Parallel || config.Playbook.Parallel

	if execOptions.Verbose {
		log.Printf("[VERBOSE] Playbook: %s", config.Playbook.Name)
		log.Printf("[VERBOSE] Execution mode: %s", map[bool]string{true: "parallel", false: "sequential"}[parallel])
		log.Printf("[VERBOSE] Dry-run: %v", execOptions.DryRun)
	}

	if execOptions.DryRun {
		fmt.Printf("\n🔍 DRY-RUN MODE - No actual changes will be made\n")
	}

	fmt.Printf("\n╔════════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║  PLAYBOOK: %-52s║\n", config.Playbook.Name)
	if parallel {
		fmt.Printf("║  MODE: Parallel Execution                                      ║\n")
	}
	fmt.Printf("╚════════════════════════════════════════════════════════════════╝\n\n")

	var results []HostResult

	if len(config.Inventory.Groups) > 0 {
		results, err = executeWithGroups(config)
		if err != nil {
			printPlaybookSummary(results, time.Since(playbookStart), err)
			return fmt.Errorf("playbook execution failed")
		}
	} else if len(config.Inventory.Hosts) > 0 {
		if parallel {
			results = executeHostsParallel(config.Inventory.Hosts, config.Playbook.Tasks)
		} else {
			results = executeHostsSequential(config.Inventory.Hosts, config.Playbook.Tasks)
		}
	} else {
		return fmt.Errorf("no hosts or groups defined in inventory")
	}

	// Check if any host failed
	hasFailure := false
	for _, result := range results {
		if !result.Success {
			hasFailure = true
			break
		}
	}

	printPlaybookSummary(results, time.Since(playbookStart), nil)

	if hasFailure {
		return fmt.Errorf("playbook execution failed")
	}

	return nil
}

func applySSHDefaults(config *Config) {
	if config.Inventory.SSHConfig == nil {
		return
	}

	sshDefaults := config.Inventory.SSHConfig

	// Apply to direct hosts
	for i := range config.Inventory.Hosts {
		applySSHDefaultsToHost(&config.Inventory.Hosts[i], sshDefaults)
	}

	// Apply to group hosts
	for i := range config.Inventory.Groups {
		for j := range config.Inventory.Groups[i].Hosts {
			applySSHDefaultsToHost(&config.Inventory.Groups[i].Hosts[j], sshDefaults)
		}
	}
}

func applySSHDefaultsToHost(host *Host, defaults *SSHConfig) {
	// Set name to hostname if name is empty
	if host.Name == "" {
		if host.Hostname != "" {
			host.Name = host.Hostname
		} else if host.Address != "" {
			host.Name = host.Address
		}
	}
	
	// Only apply defaults if host doesn't have its own value
	if host.User == "" && defaults.User != "" {
		host.User = defaults.User
	}
	if host.Password == "" && defaults.Password != "" {
		host.Password = defaults.Password
	}
	if host.KeyFile == "" && defaults.KeyFile != "" {
		host.KeyFile = defaults.KeyFile
	}
	if host.KeyPassword == "" && defaults.KeyPassword != "" {
		host.KeyPassword = defaults.KeyPassword
	}
	if !host.UseAgent && defaults.UseAgent {
		host.UseAgent = defaults.UseAgent
	}
	if host.Port == 0 && defaults.Port != 0 {
		host.Port = defaults.Port
	}
}

func main() {
	dryRun := flag.Bool("dry-run", false, "Run in dry-run mode (don't execute commands)")
	dryRunShort := flag.Bool("n", false, "Run in dry-run mode (shorthand)")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	verboseShort := flag.Bool("v", false, "Enable verbose logging (shorthand)")
	parallel := flag.Bool("parallel", false, "Execute on all hosts in parallel")
	parallelShort := flag.Bool("p", false, "Execute on all hosts in parallel (shorthand)")
	progress := flag.Bool("progress", false, "Show progress indicators for long-running tasks")
	noColor := flag.Bool("no-color", false, "Disable colored output")

	flag.Parse()

	execOptions.DryRun = *dryRun || *dryRunShort
	execOptions.Verbose = *verbose || *verboseShort
	execOptions.Parallel = *parallel || *parallelShort
	execOptions.Progress = *progress
	execOptions.NoColor = *noColor

	if flag.NArg() < 1 {
		fmt.Println("Usage: sshot [options] <playbook.yml>")
		fmt.Println("\nOptions:")
		fmt.Println("  -n, --dry-run     Run in dry-run mode (don't execute commands)")
		fmt.Println("  -v, --verbose     Enable verbose logging")
		fmt.Println("  -p, --parallel    Execute on all hosts in parallel")
		fmt.Println("  --progress        Show progress indicators for long-running tasks")
		fmt.Println("  --no-color        Disable colored output")
		fmt.Println("\nExample:")
		fmt.Println("  sshot playbook.yml")
		fmt.Println("  sshot -n playbook.yml          # Dry-run")
		fmt.Println("  sshot -v playbook.yml          # Verbose")
		fmt.Println("  sshot -p playbook.yml          # Parallel")
		fmt.Println("  sshot --progress playbook.yml  # With progress")
		fmt.Println("  sshot -nvp playbook.yml        # All options")
		os.Exit(1)
	}

	playbookPath := flag.Arg(0)
	if _, err := os.Stat(playbookPath); os.IsNotExist(err) {
		log.Fatalf("Playbook file not found: %s", playbookPath)
	}

	if execOptions.Verbose {
		log.Printf("[VERBOSE] Starting sshot")
		log.Printf("[VERBOSE] Playbook path: %s", playbookPath)
		log.Printf("[VERBOSE] Options: dry-run=%v, verbose=%v, parallel=%v, progress=%v, no-color=%v",
			execOptions.DryRun, execOptions.Verbose, execOptions.Parallel, execOptions.Progress, execOptions.NoColor)
	}

	if err := RunPlaybook(playbookPath); err != nil {
		log.Fatalf("Playbook execution failed: %v", err)
	}
}
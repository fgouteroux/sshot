package main

import (
	"io"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type Config struct {
	Inventory Inventory `yaml:"inventory"`
	Playbook  Playbook  `yaml:"playbook"`
}

type ExecutionOptions struct {
	DryRun   bool
	Verbose  bool
	Progress bool
	NoColor  bool
}

type Inventory struct {
	Hosts     []Host     `yaml:"hosts,omitempty"`
	Groups    []Group    `yaml:"groups,omitempty"`
	SSHConfig *SSHConfig `yaml:"ssh_config,omitempty"`
}

type SSHConfig struct {
	User               string `yaml:"user,omitempty"`
	Password           string `yaml:"password,omitempty"`
	KeyFile            string `yaml:"key_file,omitempty"`
	KeyPassword        string `yaml:"key_password,omitempty"`
	UseAgent           bool   `yaml:"use_agent,omitempty"`
	Port               int    `yaml:"port,omitempty"`
	StrictHostKeyCheck bool   `yaml:"strict_host_key_check,omitempty"`
}

type Group struct {
	Name      string   `yaml:"name"`
	Hosts     []Host   `yaml:"hosts"`
	Parallel  bool     `yaml:"parallel,omitempty"`
	Order     int      `yaml:"order,omitempty"`
	DependsOn []string `yaml:"depends_on,omitempty"`
}

type Host struct {
	Name               string            `yaml:"name"`
	Address            string            `yaml:"address,omitempty"`
	Hostname           string            `yaml:"hostname,omitempty"`
	Port               int               `yaml:"port"`
	User               string            `yaml:"user"`
	Password           string            `yaml:"password,omitempty"`
	KeyFile            string            `yaml:"key_file,omitempty"`
	KeyPassword        string            `yaml:"key_password,omitempty"`
	UseAgent           bool              `yaml:"use_agent,omitempty"`
	StrictHostKeyCheck bool              `yaml:"strict_host_key_check,omitempty"`
	Vars               map[string]string `yaml:"vars,omitempty"`
}

type Playbook struct {
	Name     string `yaml:"name"`
	Parallel bool   `yaml:"parallel,omitempty"`
	Tasks    []Task `yaml:"tasks"`
}

type Task struct {
	Name         string            `yaml:"name"`
	Command      string            `yaml:"command,omitempty"`
	Script       string            `yaml:"script,omitempty"`
	Copy         *CopyTask         `yaml:"copy,omitempty"`
	Shell        string            `yaml:"shell,omitempty"`
	Sudo         bool              `yaml:"sudo,omitempty"`
	When         string            `yaml:"when,omitempty"`
	Register     string            `yaml:"register,omitempty"`
	IgnoreError  bool              `yaml:"ignore_error,omitempty"`
	Vars         map[string]string `yaml:"vars,omitempty"`
	DependsOn    []string          `yaml:"depends_on,omitempty"`
	WaitFor      string            `yaml:"wait_for,omitempty"`
	Retries      int               `yaml:"retries,omitempty"`
	RetryDelay   int               `yaml:"retry_delay,omitempty"`
	Timeout      int               `yaml:"timeout,omitempty"`
	UntilSuccess bool              `yaml:"until_success,omitempty"`
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

type HostResult struct {
	Host    Host
	Success bool
	Error   error
	Output  string
}

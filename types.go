package main

import (
	"io"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

var runOnceTasks = struct {
	sync.RWMutex
	executed map[string]bool
}{
	executed: make(map[string]bool),
}

type Config struct {
	Inventory Inventory `yaml:"inventory"`
	Playbook  Playbook  `yaml:"playbook"`
}

// InventoryConfig represents a standalone inventory file
type InventoryConfig struct {
	Hosts     []Host     `yaml:"hosts,omitempty"`
	Groups    []Group    `yaml:"groups,omitempty"`
	SSHConfig *SSHConfig `yaml:"ssh_config,omitempty"`
}

type FactCollector struct {
	Name    string `yaml:"name"`
	Command string `yaml:"command"`
	Sudo    bool   `yaml:"sudo,omitempty"`
}

type FactsConfig struct {
	Collectors []FactCollector `yaml:"collectors,omitempty"`
}

// PlaybookConfig represents a standalone playbook file
type PlaybookConfig struct {
	Name     string      `yaml:"name"`
	Parallel bool        `yaml:"parallel,omitempty"`
	Facts    FactsConfig `yaml:"facts,omitempty"`
	Tasks    []Task      `yaml:"tasks"`
}

type ExecutionOptions struct {
	DryRun        bool
	Verbose       bool
	Progress      bool
	NoColor       bool
	FullOutput    bool
	InventoryFile string
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
	StrictHostKeyCheck *bool  `yaml:"strict_host_key_check,omitempty"`
}

type Group struct {
	Name      string   `yaml:"name"`
	Hosts     []Host   `yaml:"hosts"`
	Parallel  bool     `yaml:"parallel,omitempty"`
	Order     int      `yaml:"order,omitempty"`
	DependsOn []string `yaml:"depends_on,omitempty"`
}

type Host struct {
	Name               string                 `yaml:"name"`
	Address            string                 `yaml:"address,omitempty"`
	Hostname           string                 `yaml:"hostname,omitempty"`
	Port               int                    `yaml:"port"`
	User               string                 `yaml:"user"`
	Password           string                 `yaml:"password,omitempty"`
	KeyFile            string                 `yaml:"key_file,omitempty"`
	KeyPassword        string                 `yaml:"key_password,omitempty"`
	UseAgent           bool                   `yaml:"use_agent,omitempty"`
	StrictHostKeyCheck *bool                  `yaml:"strict_host_key_check,omitempty"`
	Vars               map[string]interface{} `yaml:"vars,omitempty"`
}

type Playbook struct {
	Name     string      `yaml:"name"`
	Parallel bool        `yaml:"parallel,omitempty"`
	Facts    FactsConfig `yaml:"facts,omitempty"`
	Tasks    []Task      `yaml:"tasks"`
}

type Task struct {
	Name             string                 `yaml:"name"`
	Command          string                 `yaml:"command,omitempty"`
	Script           string                 `yaml:"script,omitempty"`
	Copy             *CopyTask              `yaml:"copy,omitempty"`
	Shell            string                 `yaml:"shell,omitempty"`
	Sudo             bool                   `yaml:"sudo,omitempty"`
	When             string                 `yaml:"when,omitempty"`
	Register         string                 `yaml:"register,omitempty"`
	OnlyGroups       []string               `yaml:"only_groups,omitempty"`
	SkipGroups       []string               `yaml:"skip_groups,omitempty"`
	LocalAction      string                 `yaml:"local_action,omitempty"`
	DelegateTo       string                 `yaml:"delegate_to,omitempty"`
	RunOnce          bool                   `yaml:"run_once,omitempty"`
	IgnoreError      bool                   `yaml:"ignore_error,omitempty"`
	Vars             map[string]interface{} `yaml:"vars,omitempty"`
	DependsOn        []string               `yaml:"depends_on,omitempty"`
	WaitFor          string                 `yaml:"wait_for,omitempty"`
	Retries          int                    `yaml:"retries,omitempty"`
	RetryDelay       int                    `yaml:"retry_delay,omitempty"`
	Timeout          int                    `yaml:"timeout,omitempty"`
	UntilSuccess     bool                   `yaml:"until_success,omitempty"`
	AllowedExitCodes []int                  `yaml:"allowed_exit_codes,omitempty"`
}

type CopyTask struct {
	Src  string `yaml:"src"`
	Dest string `yaml:"dest"`
	Mode string `yaml:"mode,omitempty"`
}

type Executor struct {
	host           Host
	client         *ssh.Client
	variables      map[string]interface{}
	registers      map[string]string
	completedTasks map[string]bool
	groupName      string
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

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFullPlaybookExecution_DryRun(t *testing.T) {
	execOptions.DryRun = true
	execOptions.Verbose = false
	defer func() {
		execOptions.DryRun = false
		execOptions.Verbose = false
	}()

	tmpDir := t.TempDir()
	playbookFile := filepath.Join(tmpDir, "playbook.yml")

	playbookContent := `
inventory:
  ssh_config:
    user: admin
    port: 22
    password: testpass
  hosts:
    - name: server1
      address: 192.168.1.10
    - name: server2
      address: 192.168.1.11
playbook:
  name: Test Deployment
  parallel: false
  tasks:
    - name: Update System
      command: apt-get update
      sudo: true
    - name: Install Package
      command: apt-get install -y nginx
      sudo: true
      retries: 3
      retry_delay: 5
    - name: Copy Config
      copy:
        src: /local/nginx.conf
        dest: /etc/nginx/nginx.conf
        mode: "0644"
    - name: Restart Service
      command: systemctl restart nginx
      sudo: true
`

	err := os.WriteFile(playbookFile, []byte(playbookContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create playbook file: %v", err)
	}

	err = RunPlaybook(playbookFile)
	if err != nil {
		t.Errorf("RunPlaybook() error = %v", err)
	}
}

func TestFullPlaybookExecution_WithGroups(t *testing.T) {
	execOptions.DryRun = true
	execOptions.Verbose = false
	defer func() {
		execOptions.DryRun = false
		execOptions.Verbose = false
	}()

	tmpDir := t.TempDir()
	playbookFile := filepath.Join(tmpDir, "playbook_groups.yml")

	playbookContent := `
inventory:
  ssh_config:
    user: admin
    password: testpass
  groups:
    - name: databases
      order: 1
      parallel: false
      hosts:
        - name: db1
          address: 192.168.1.20
        - name: db2
          address: 192.168.1.21
    - name: webservers
      order: 2
      parallel: true
      depends_on: [databases]
      hosts:
        - name: web1
          address: 192.168.1.30
        - name: web2
          address: 192.168.1.31
playbook:
  name: Multi-tier Deployment
  tasks:
    - name: Health Check
      command: echo "healthy"
    - name: Deploy Application
      command: deploy.sh
      retries: 2
`

	err := os.WriteFile(playbookFile, []byte(playbookContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create playbook file: %v", err)
	}

	err = RunPlaybook(playbookFile)
	if err != nil {
		t.Errorf("RunPlaybook() with groups error = %v", err)
	}
}

func TestFullPlaybookExecution_WithConditionals(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	tmpDir := t.TempDir()
	playbookFile := filepath.Join(tmpDir, "playbook_conditional.yml")

	playbookContent := `
inventory:
  ssh_config:
    user: admin
    password: testpass
  hosts:
    - name: ubuntu-server
      address: 192.168.1.40
      vars:
        os: ubuntu
        env: production
playbook:
  name: Conditional Tasks
  tasks:
    - name: Ubuntu Specific Task
      command: apt-get update
      when: "{{.os}} == 'ubuntu'"
    - name: Production Only Task
      command: deploy-prod.sh
      when: "{{.env}} == 'production'"
    - name: Skipped Task
      command: yum update
      when: "{{.os}} == 'centos'"
`

	err := os.WriteFile(playbookFile, []byte(playbookContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create playbook file: %v", err)
	}

	err = RunPlaybook(playbookFile)
	if err != nil {
		t.Errorf("RunPlaybook() with conditionals error = %v", err)
	}
}

func TestFullPlaybookExecution_WithVariables(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	tmpDir := t.TempDir()
	playbookFile := filepath.Join(tmpDir, "playbook_vars.yml")

	playbookContent := `
inventory:
  ssh_config:
    user: admin
    password: testpass
  hosts:
    - name: app-server
      address: 192.168.1.50
      vars:
        app_name: myapp
        app_port: "8080"
        app_path: /opt/myapp
playbook:
  name: Variable Substitution
  tasks:
    - name: Create App Directory
      command: mkdir -p {{.app_path}}
    - name: Deploy App
      command: deploy {{.app_name}} --port {{.app_port}}
      vars:
        version: "1.2.3"
    - name: Check Version
      command: echo {{.version}}
      register: version_output
`

	err := os.WriteFile(playbookFile, []byte(playbookContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create playbook file: %v", err)
	}

	err = RunPlaybook(playbookFile)
	if err != nil {
		t.Errorf("RunPlaybook() with variables error = %v", err)
	}
}

func TestFullPlaybookExecution_WithDependencies(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	tmpDir := t.TempDir()
	playbookFile := filepath.Join(tmpDir, "playbook_deps.yml")

	playbookContent := `
inventory:
  ssh_config:
    user: admin
    password: testpass
  hosts:
    - name: build-server
      address: 192.168.1.60
playbook:
  name: Dependent Tasks
  tasks:
    - name: Install Dependencies
      command: apt-get install -y build-essential
    - name: Clone Repository
      command: git clone https://github.com/user/repo.git
      depends_on: [Install Dependencies]
    - name: Build Application
      command: make build
      depends_on: [Clone Repository]
    - name: Run Tests
      command: make test
      depends_on: [Build Application]
`

	err := os.WriteFile(playbookFile, []byte(playbookContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create playbook file: %v", err)
	}

	err = RunPlaybook(playbookFile)
	if err != nil {
		t.Errorf("RunPlaybook() with dependencies error = %v", err)
	}
}

func TestFullPlaybookExecution_ParallelMode(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	tmpDir := t.TempDir()
	playbookFile := filepath.Join(tmpDir, "playbook_parallel.yml")

	playbookContent := `
inventory:
  ssh_config:
    user: admin
    password: testpass
  hosts:
    - name: node1
      address: 192.168.1
    - name: node2
      address: 192.168.1
    - name: node3
      address: 192.168.1
playbook:
  name: Parallel Deployment
  parallel: true
  tasks:
    - name: Deploy Service
      command: systemctl restart myservice
    - name: Health Check
      command: curl -f http://localhost:8080/health
      retries: 5
      retry_delay: 2
`

	err := os.WriteFile(playbookFile, []byte(playbookContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create playbook file: %v", err)
	}

	err = RunPlaybook(playbookFile)
	if err != nil {
		t.Errorf("RunPlaybook() in parallel mode error = %v", err)
	}
}

func TestFullPlaybookExecution_WithScripts(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	tmpDir := t.TempDir()

	// Create a test script
	scriptPath := filepath.Join(tmpDir, "setup.sh")
	scriptContent := `#!/bin/bash
echo "Setting up environment"
export VAR1=value1
echo "Done"
`
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0700)
	if err != nil {
		t.Fatalf("Failed to create script file: %v", err)
	}

	playbookFile := filepath.Join(tmpDir, "playbook_script.yml")
	playbookContent := `
inventory:
  ssh_config:
    user: admin
    password: testpass
  hosts:
    - name: script-server
      address: 192.168.
playbook:
  name: Script Execution
  tasks:
    - name: Run Setup Script
      script: ` + scriptPath + `
      sudo: true
    - name: Verify Setup
      command: echo "Setup complete"
`

	err = os.WriteFile(playbookFile, []byte(playbookContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create playbook file: %v", err)
	}

	err = RunPlaybook(playbookFile)
	if err != nil {
		t.Errorf("RunPlaybook() with script error = %v", err)
	}
}

func TestFullPlaybookExecution_WithWaitFor(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	tmpDir := t.TempDir()
	playbookFile := filepath.Join(tmpDir, "playbook_waitfor.yml")

	playbookContent := `
inventory:
  ssh_config:
    user: admin
    password: testpass
  hosts:
    - name: service-server
      address: 192.168.
playbook:
  name: Service Management with Wait
  tasks:
    - name: Start Service
      command: systemctl start myservice
      sudo: true
    - name: Wait for Port
      wait_for: port:8080
    - name: Wait for Service
      wait_for: service:myservice
    - name: Verify Service
      command: curl http://localhost:8080
`

	err := os.WriteFile(playbookFile, []byte(playbookContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create playbook file: %v", err)
	}

	err = RunPlaybook(playbookFile)
	if err != nil {
		t.Errorf("RunPlaybook() with wait_for error = %v", err)
	}
}

func TestFullPlaybookExecution_ComplexScenario(t *testing.T) {
	execOptions.DryRun = true
	execOptions.Verbose = true
	defer func() {
		execOptions.DryRun = false
		execOptions.Verbose = false
	}()

	tmpDir := t.TempDir()
	playbookFile := filepath.Join(tmpDir, "playbook_complex.yml")

	playbookContent := `
inventory:
  ssh_config:
    user: deploy
    port: 22
    password: testpass
  groups:
    - name: database
      order: 1
      hosts:
        - name: db-master
          address: 192.168.1.200
          vars:
            role: master
        - name: db-slave
          address: 192.168.1.201
          vars:
            role: slave
    - name: application
      order: 2
      parallel: true
      depends_on: [database]
      hosts:
        - name: app1
          address: 192.168.1.210
        - name: app2
          address: 192.168.1.211
    - name: loadbalancer
      order: 3
      depends_on: [application]
      hosts:
        - name: lb1
          address: 192.168.1.220
playbook:
  name: Complex Multi-tier Deployment
  tasks:
    - name: Pre-deployment Check
      command: echo "Starting deployment"
    - name: Stop Services
      command: systemctl stop myapp
      ignore_error: true
    - name: Backup Database
      command: backup.sh
      when: "{{.role}} == 'master'"
      timeout: 300
    - name: Update Application
      command: deploy.sh --version {{.version}}
      vars:
        version: "2.0.0"
      retries: 3
      retry_delay: 10
    - name: Start Services
      command: systemctl start myapp
      sudo: true
    - name: Wait for Service
      wait_for: port:8080
    - name: Health Check
      command: curl -f http://localhost:8080/health
      retries: 5
      retry_delay: 3
      until_success: true
    - name: Post-deployment Verification
      command: test-suite.sh
      register: test_results
`

	err := os.WriteFile(playbookFile, []byte(playbookContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create playbook file: %v", err)
	}

	err = RunPlaybook(playbookFile)
	if err != nil {
		t.Errorf("RunPlaybook() complex scenario error = %v", err)
	}
}

func TestFullPlaybookExecution_FailedGroupDependency(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	tmpDir := t.TempDir()
	playbookFile := filepath.Join(tmpDir, "playbook_fail_dep.yml")

	playbookContent := `
inventory:
  ssh_config:
    user: admin
    password: testpass
  groups:
    - name: group2
      order: 2
      depends_on: [nonexistent-group]
      hosts:
        - name: host1
          address: 192.168.1.1
          password: testpass
playbook:
  name: Failed Dependency Test
  tasks:
    - name: Test Task
      command: echo test
`

	err := os.WriteFile(playbookFile, []byte(playbookContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create playbook file: %v", err)
	}

	err = RunPlaybook(playbookFile)
	if err == nil {
		t.Error("RunPlaybook() should fail with missing dependency")
	}
}

func TestFullPlaybookExecution_WithProgress(t *testing.T) {
	execOptions.DryRun = true
	execOptions.Progress = true
	defer func() {
		execOptions.DryRun = false
		execOptions.Progress = false
	}()

	tmpDir := t.TempDir()
	playbookFile := filepath.Join(tmpDir, "playbook_progress.yml")

	playbookContent := `
inventory:
  ssh_config:
    user: admin
    password: testpass
  hosts:
    - name: progress-server
      address: 192.168.
playbook:
  name: Progress Indicator Test
  tasks:
    - name: Long Running Task
      command: sleep 5 && echo done
      timeout: 10
`

	err := os.WriteFile(playbookFile, []byte(playbookContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create playbook file: %v", err)
	}

	err = RunPlaybook(playbookFile)
	if err != nil {
		t.Errorf("RunPlaybook() with progress error = %v", err)
	}
}

func TestFullPlaybookExecution_WithNoColor(t *testing.T) {
	execOptions.DryRun = true
	execOptions.NoColor = true
	defer func() {
		execOptions.DryRun = false
		execOptions.NoColor = false
	}()

	tmpDir := t.TempDir()
	playbookFile := filepath.Join(tmpDir, "playbook_nocolor.yml")

	playbookContent := `
inventory:
  ssh_config:
    user: admin
    password: testpass
  hosts:
    - name: color-test-server
      address: 192.168.
playbook:
  name: No Color Test
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
		t.Errorf("RunPlaybook() with no-color error = %v", err)
	}

	// Verify color function returns empty string
	colorResult := color(ColorRed)
	if colorResult != "" {
		t.Errorf("color() should return empty string when NoColor=true, got: %q", colorResult)
	}
}

func TestFullPlaybookExecution_MixedHostsAndGroups(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	tmpDir := t.TempDir()
	playbookFile := filepath.Join(tmpDir, "playbook_mixed.yml")

	// Test with groups present - groups should take precedence
	playbookContent := `
inventory:
  ssh_config:
    user: admin
    password: testpass
  hosts:
    - name: standalone1
      address: 192.168.1
  groups:
    - name: grouped-servers
      order: 1
      hosts:
        - name: grouped1
          address: 192.168.1.110
playbook:
  name: Mixed Inventory Test
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
		t.Errorf("RunPlaybook() with mixed inventory error = %v", err)
	}
}

func TestFullPlaybookExecution_EmptyTasks(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	tmpDir := t.TempDir()
	playbookFile := filepath.Join(tmpDir, "playbook_empty.yml")

	playbookContent := `
inventory:
  ssh_config:
    user: admin
    password: testpass
  hosts:
    - name: empty-task-server
      address: 192.168.1
playbook:
  name: Empty Tasks Test
  tasks: []
`

	err := os.WriteFile(playbookFile, []byte(playbookContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create playbook file: %v", err)
	}

	err = RunPlaybook(playbookFile)
	if err != nil {
		t.Errorf("RunPlaybook() with empty tasks error = %v", err)
	}
}

func TestFullPlaybookExecution_HostnameOnly(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	tmpDir := t.TempDir()
	playbookFile := filepath.Join(tmpDir, "playbook_hostname.yml")

	playbookContent := `
inventory:
  ssh_config:
    user: admin
    password: testpass
  hosts:
    - hostname: example
playbook:
  name: Hostname Only Test
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
		t.Errorf("RunPlaybook() with hostname only error = %v", err)
	}
}

func TestFullPlaybookExecution_AllSSHOptions(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	tmpDir := t.TempDir()
	playbookFile := filepath.Join(tmpDir, "playbook_ssh_opts.yml")

	playbookContent := `
inventory:
  ssh_config:
    user: default_user
    password: default_pass
    port: 2222
    strict_host_key_check: false
  hosts:
    - name: ssh-test-server
      address: 192.168.1.130
playbook:
  name: SSH Options Test
  tasks:
    - name: Test Connection
      command: echo "connected"
`

	err := os.WriteFile(playbookFile, []byte(playbookContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create playbook file: %v", err)
	}

	err = RunPlaybook(playbookFile)
	if err != nil {
		t.Errorf("RunPlaybook() with SSH options error = %v", err)
	}
}

func TestFullPlaybookExecution_TaskWithAllOptions(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	tmpDir := t.TempDir()
	playbookFile := filepath.Join(tmpDir, "playbook_all_opts.yml")

	playbookContent := `
inventory:
  ssh_config:
    user: admin
    password: testpass
  hosts:
    - name: all-opts-server
      address: 192.168.1
      vars:
        env: test
playbook:
  name: All Task Options Test
  tasks:
    - name: Task with All Options
      command: echo "test"
      sudo: true
      when: "{{.env}} == 'test'"
      register: output
      ignore_error: true
      vars:
        task_var: task_value
      depends_on: []
      retries: 2
      retry_delay: 3
      timeout: 30
      until_success: false
`

	err := os.WriteFile(playbookFile, []byte(playbookContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create playbook file: %v", err)
	}

	err = RunPlaybook(playbookFile)
	if err != nil {
		t.Errorf("RunPlaybook() with all task options error = %v", err)
	}
}

func TestIntegration_MultipleGroupOrders(t *testing.T) {
	execOptions.DryRun = true
	defer func() {
		execOptions.DryRun = false
	}()

	tmpDir := t.TempDir()
	playbookFile := filepath.Join(tmpDir, "playbook_orders.yml")

	playbookContent := `
inventory:
  ssh_config:
    user: admin
    password: testpass
  groups:
    - name: group-c
      order: 3
      hosts:
        - name: hostc
          address: 192.168.1.153
    - name: group-a
      order: 1
      hosts:
        - name: hosta
          address: 192.168.1.151
    - name: group-b
      order: 2
      hosts:
        - name: hostb
          address: 192.168.1.152
playbook:
  name: Order Test
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
		t.Errorf("RunPlaybook() with multiple group orders error = %v", err)
	}
}

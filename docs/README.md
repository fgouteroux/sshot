# SSHOT - SSH Orchestrator Tool

SSHOT (SSH Orchestrator Tool) is a lightweight, Ansible-inspired tool designed for sysadmins who need straightforward SSH orchestration without Python dependency headaches. Built with Go for portability and simplicity, it uses familiar YAML playbooks—perfect for daily administrative tasks.

[![Go Report Card](https://goreportcard.com/badge/github.com/fgouteroux/sshot)](https://goreportcard.com/report/github.com/fgouteroux/sshot)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/fgouteroux/sshot)](https://github.com/fgouteroux/sshot/releases)

## Table of Contents

- [Why SSHOT?](#why-sshot)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Core Concepts](#core-concepts)
- [Usage Examples](#usage-examples)
- [Command Line Reference](#command-line-reference)
- [Configuration Reference](#configuration-reference)
- [Advanced Features](#advanced-features)
- [Troubleshooting](#troubleshooting)

## Why SSHOT?

If you're a sysadmin who loves Ansible's YAML approach but sometimes finds Python dependencies challenging, SSHOT might be for you.

**SSHOT is NOT a replacement for Ansible** - it doesn't try to be. Ansible is a comprehensive automation platform with an extensive ecosystem. SSHOT is simply a focused helper tool for sysadmins who need straightforward SSH orchestration.

### Key Benefits

- 🪶 **No Python headaches** - Single Go binary, no dependencies, no virtualenvs, no pip issues
- 🎯 **Sysadmin-focused** - Built for daily SSH tasks, not enterprise-wide automation
- ⚡ **Portable** - Copy one binary, run anywhere (Linux, macOS, even on edge devices)
- 📝 **Familiar syntax** - If you know Ansible YAML, you already know SSHOT
- 🚀 **Fast** - Go's performance for quick task execution

## Installation

### From Release Binary
```bash
# Download from GitHub releases
wget https://github.com/fgouteroux/sshot/releases/latest/download/sshot_Linux_x86_64.tar.gz
tar xzf sshot_Linux_x86_64.tar.gz
sudo mv sshot /usr/local/bin/
```

### Using Go Install
```bash
go install github.com/fgouteroux/sshot@latest
```

### Build from Source
```bash
git clone https://github.com/fgouteroux/sshot.git
cd sshot
go build -o sshot
sudo mv sshot /usr/local/bin/
```

## Quick Start

### 1. Create a Simple Inventory
```yaml
# inventory.yml
ssh_config:
  user: admin
  key_file: ~/.ssh/id_rsa
  port: 22

hosts:
  - name: web1
    address: 192.168.1.10
  - name: web2
    address: 192.168.1.11
```

### 2. Create a Basic Playbook
```yaml
# playbook.yml
name: Deploy Application
tasks:
  - name: Update system
    command: apt-get update
    sudo: true
    
  - name: Install nginx
    command: apt-get install -y nginx
    sudo: true
    
  - name: Start nginx
    command: systemctl start nginx
    sudo: true
```

### 3. Run SSHOT
```bash
sshot -i inventory.yml playbook.yml
```

## Core Concepts

### Playbooks and Inventory

SSHOT uses two key YAML files to define your automation:

1. **Inventory** - Defines servers, groups, and SSH connection details
2. **Playbook** - Defines tasks to execute on servers

You can use separate files or combine them into a single file.

### Inventory Structure

The inventory defines:

- SSH configuration defaults
- Hosts with their connection details
- Host grouping and execution order
- Variables for use in tasks

### Playbook Structure

The playbook defines:

- A sequence of tasks to run
- Task dependencies and conditions
- Execution options (parallel or sequential)
- Retry logic and error handling

### Task Types

SSHOT supports multiple task types:

- **Command** - Execute shell commands
- **Script** - Upload and run local scripts
- **Copy** - Transfer files to remote hosts
- **Wait For** - Wait for a condition to be met

## Usage Examples

### Basic Example

This example updates packages on a single server:
```yaml
# inventory.yml
ssh_config:
  user: admin
  key_file: ~/.ssh/id_rsa

hosts:
  - name: server1
    address: 192.168.1.100

# playbook.yml
name: Update Packages
tasks:
  - name: Update package lists
    command: apt-get update
    sudo: true
  
  - name: Upgrade packages
    command: apt-get upgrade -y
    sudo: true
```

Run it:
```bash
sshot -i inventory.yml playbook.yml
```

### Web Server Deployment

This example deploys a web server with configuration:
```yaml
# inventory.yml
ssh_config:
  user: admin
  key_file: ~/.ssh/id_rsa

hosts:
  - name: webserver
    address: 192.168.1.100

# playbook.yml
name: Deploy Web Server
tasks:
  - name: Install nginx
    command: apt-get install -y nginx
    sudo: true
    
  - name: Copy configuration
    copy:
      src: ./nginx.conf
      dest: /etc/nginx/nginx.conf
      mode: "0644"
    sudo: true
    
  - name: Start nginx
    command: systemctl restart nginx
    sudo: true
    
  - name: Wait for service
    wait_for: port:80
    
  - name: Verify service
    command: curl -s http://localhost
    register: curl_output
```

Run it:
```bash
sshot -i inventory.yml playbook.yml
```

### Multi-tier Application Deployment

This example uses groups for ordered deployment:
```yaml
# inventory.yml
ssh_config:
  user: admin
  key_file: ~/.ssh/id_rsa

groups:
  - name: database
    order: 1
    hosts:
      - name: db1
        address: 192.168.1.10
        
  - name: application
    order: 2
    depends_on: [database]
    hosts:
      - name: app1
        address: 192.168.1.20
      - name: app2
        address: 192.168.1.21
        
  - name: loadbalancer
    order: 3
    depends_on: [application]
    hosts:
      - name: lb1
        address: 192.168.1.30

# playbook.yml
name: Deploy Application Stack
tasks:
  - name: Update system
    command: apt-get update
    sudo: true
    
  - name: Install required packages
    command: apt-get install -y {% raw %}{{ .packages }}{% endraw %}
    sudo: true
    vars:
      packages: {% raw %}{{ .role_packages }}{% endraw %}
    
  - name: Start services
    command: systemctl restart {% raw %}{{ .service }}{% endraw %}
    sudo: true
    vars:
      service: {% raw %}{{ .role_service }}{% endraw %}
    
  - name: Health check
    command: {% raw %}{{ .health_cmd }}{% endraw %}
    retries: 5
    retry_delay: 2
```

Run it:
```bash
sshot -i inventory.yml playbook.yml
```

### Conditional Task Execution

This example shows conditional tasks based on host variables:
```yaml
# inventory.yml
ssh_config:
  user: admin
  key_file: ~/.ssh/id_rsa

hosts:
  - name: ubuntu-server
    address: 192.168.1.10
    vars:
      os: ubuntu
      version: "20.04"
      
  - name: centos-server
    address: 192.168.1.11
    vars:
      os: centos
      version: "8"

# playbook.yml
name: OS-specific Updates
tasks:
  - name: Update Ubuntu
    command: apt-get update
    sudo: true
    when: {% raw %}"{{.os}} == ubuntu"{% endraw %}
    
  - name: Update CentOS
    command: yum update -y
    sudo: true
    when: {% raw %}"{{.os}} == centos"{% endraw %}
    
  - name: Install common tools
    command: {% raw %}"{{.os}} == ubuntu && apt-get install -y vim || yum install -y vim"{% endraw %}
    sudo: true
```

Run it:
```bash
sshot -i inventory.yml playbook.yml
```

## Command Line Reference
```bash
sshot [options] <playbook.yml>
```

### Options

| Option | Description |
|--------|-------------|
| `-i, --inventory <file>` | Path to inventory file (if separate from playbook) |
| `-n, --dry-run` | Run in dry-run mode (simulate without executing) |
| `-v, --verbose` | Enable verbose logging |
| `-p, --progress` | Show progress indicators for long-running tasks |
| `-f, --full-output` | Show complete command output without truncation |
| `--no-color` | Disable colored output |

### Examples

**Basic execution:**
```bash
sshot playbook.yml
```

**With separate inventory:**
```bash
sshot -i inventory.yml playbook.yml
```

**Dry-run mode with verbose output:**
```bash
sshot -n -v -i inventory.yml playbook.yml
```

**With progress indicators:**
```bash
sshot --progress -i inventory.yml playbook.yml
```

**With full output:**
```bash
sshot -f -i inventory.yml playbook.yml
```

## Configuration Reference

### Inventory

#### SSH Configuration
```yaml
ssh_config:
  user: admin                # Default SSH user
  password: secret           # Default password (not recommended)
  key_file: ~/.ssh/id_rsa    # Path to SSH key
  key_password: passphrase   # SSH key passphrase
  port: 22                   # Default SSH port
  use_agent: true            # Use SSH agent for auth
  strict_host_key_check: true  # Verify host keys
```

#### Hosts
```yaml
hosts:
  - name: server1                   # Name for display
    address: 192.168.1.10           # IP address
    hostname: server1.example.com   # DNS hostname (alternative to address)
    user: admin                     # Override default user
    password: secret                # Override default password
    key_file: ~/.ssh/custom_key     # Override default key file
    port: 2222                      # Override default port
    vars:                           # Host variables
      role: webserver
      env: production
```

#### Groups
```yaml
groups:
  - name: webservers                # Group name
    order: 1                        # Execution order
    parallel: true                  # Execute hosts in parallel
    depends_on: [databases]         # Group dependencies
    hosts:
      - name: web1
        address: 192.168.1.10
      - name: web2
        address: 192.168.1.11
```

### Playbook

#### Basic Structure
```yaml
name: My Playbook                   # Playbook name
parallel: false                     # Global parallel execution setting

tasks:                              # List of tasks
  - name: Task 1                    # Task name
    command: echo "Hello"           # Command to execute
```

#### Task Types

**Command Task:**
```yaml
- name: Execute command
  command: service nginx restart
  sudo: true                        # Run with sudo
```

**Shell Task:**
```yaml
- name: Execute shell command
  shell: find /var/log -name "*.log" | xargs ls -la
```

**Script Task:**
```yaml
- name: Run local script
  script: ./scripts/setup.sh        # Local script path
```

**Copy Task:**
```yaml
- name: Copy file
  copy:
    src: ./local/file.txt           # Local file path
    dest: /remote/path/file.txt     # Remote destination
    mode: "0644"                    # File permissions
```

**Wait For Task:**
```yaml
- name: Wait for port
  wait_for: port:8080               # Wait for port to be available
```

#### Task Options
```yaml
- name: Complex task example
  command: deploy.sh
  sudo: true                        # Run with sudo
  when: {% raw %}"{{.env}} == production"{% endraw %}  # Condition for execution
  register: deploy_output           # Store output in variable
  ignore_error: true                # Continue on error
  vars:                             # Task variables
    version: "2.0"
  depends_on: [Previous Task]       # Task dependencies
  retries: 3                        # Retry count
  retry_delay: 5                    # Seconds between retries
  timeout: 60                       # Task timeout in seconds
  until_success: true               # Retry until success
  allowed_exit_codes: [0, 1]        # Accept these exit codes as success
```

## Advanced Features

### Facts Collection in SSHOT

#### Overview

The facts collection feature allows you to gather system information from remote hosts before executing tasks. This information (facts) can then be used in your tasks for conditional execution or dynamic command generation, similar to Ansible facts.

Facts are collected using configurable commands that output JSON data. By default, you can use tools like Puppet's Facter to gather detailed system information.

#### Configuration

Facts collection is configured in the `facts` section of your playbook:
```yaml
playbook:
  name: My Playbook
  facts:
    collectors:
      - name: puppet_facts
        command: facter --json
        sudo: true
      - name: app_status
        command: /usr/local/bin/app-status.sh --json
        sudo: false
  tasks:
    - name: OS-specific Task
      command: echo "Running on {% raw %}{{.puppet_facts.os.name}}{% endraw %}"
      when: "{% raw %}{{.puppet_facts.os.family}} == RedHat{% endraw %}"
```

#### Collector Configuration

Each collector is defined with the following properties:

| Property | Description | Required |
|----------|-------------|----------|
| `name` | Name of the fact collection, used to access the facts | Yes |
| `command` | Command to execute that returns JSON data | Yes |
| `sudo` | Whether to run the command with sudo | No (default: false) |

#### Using Facts in Tasks

Facts are available as variables in your tasks, using the collector name as the prefix:

##### Basic Usage
```yaml
- name: Show OS Information
  command: echo "Running on {% raw %}{{.puppet_facts.os.name}} {{.puppet_facts.os.release.full}}{% endraw %}"
```

##### Conditional Execution
```yaml
- name: Debian-specific Task
  command: apt-get update
  when: "{% raw %}{{.puppet_facts.os.family}} == Debian{% endraw %}"
  
- name: RedHat-specific Task
  command: yum update
  when: "{% raw %}{{.puppet_facts.os.family}} == RedHat{% endraw %}"
```

##### Nested Facts

Facts can have nested structures, which can be accessed using dot notation:
```yaml
- name: Show Memory Information
  command: echo "Total memory: {% raw %}{{.puppet_facts.memory.system.total}}{% endraw %}"
```

#### Using Puppet Facter

[Facter](https://puppet.com/docs/puppet/latest/facter.html) is a system profiling library from Puppet that collects facts about the system it runs on. It's an excellent tool for gathering comprehensive system information.

##### Installing Facter

If you're not using the full Puppet agent, you can install just Facter:

###### On Debian/Ubuntu:
```bash
sudo apt-get install facter
```

###### On RedHat/CentOS:
```bash
sudo yum install facter
```

##### Standalone Installation:
```bash
# Download and install Puppet's release package
wget https://apt.puppetlabs.com/puppet7-release-focal.deb
sudo dpkg -i puppet7-release-focal.deb
sudo apt-get update
sudo apt-get install facter
```

### Using Facter with SSHOT

Once Facter is installed, you can use it in your facts collectors:
```yaml
facts:
  collectors:
    - name: system_facts
      command: /opt/puppetlabs/bin/facter --json
      sudo: true
```

Note that Facter is typically installed at `facter`. You may need to specify the full path when using sudo, as shown above.

#### Custom Fact Collectors

You can create your own custom fact collectors by writing scripts that output JSON data:

#### Example: Custom Application Status Collector

Create a script that outputs JSON:
```bash
#!/bin/bash
# /usr/local/bin/app-status.sh
echo '{'
echo '  "version": "1.2.3",'
echo '  "status": "running",'
echo '  "connections": 42,'
echo '  "uptime": "3d 2h 15m"'
echo '}'
```

Then use it in your playbook:
```yaml
facts:
  collectors:
    - name: app_status
      command: /usr/local/bin/app-status.sh
      sudo: false
```

Access the facts in your tasks:
```yaml
- name: Show Application Status
  command: echo "App version {% raw %}{{.app_status.version}} is {{.app_status.status}}{% endraw %}"
  
- name: Restart if Connections Too High
  command: systemctl restart myapp
  when: "{% raw %}{{.app_status.connections}}{% endraw %} > 100"
```

#### Troubleshooting

##### Command Not Found

If you get a "command not found" error when using Facter with sudo, make sure to use the full path to the Facter executable:
```yaml
command: facter --json
```

##### JSON Parsing Errors

The output of your collector commands must be valid JSON. If you're creating a custom collector script, make sure it outputs properly formatted JSON.

To test your JSON output:
```bash
/usr/local/bin/my-collector.sh | jq
```

If jq reports errors, your JSON is not valid.

##### Accessing Facts in Templates

If you have complex nested facts, you can use the dot notation to access nested values:
```
{% raw %}{{.puppet_facts.networking.interfaces.eth0.ip}}{% endraw %}
```

#### Examples

##### Basic System Information Playbook
```yaml
inventory:
  ssh_config:
    user: admin
    key_file: ~/.ssh/id_rsa
  hosts:
    - name: server1
      address: 192.168.1.10
      
playbook:
  name: System Information
  facts:
    collectors:
      - name: system
        command: facter --json
        sudo: true
  tasks:
    - name: Show System Information
      command: echo "Host: {% raw %}{{.system.networking.hostname}}, OS: {{.system.os.name}} {{.system.os.release.full}}, CPU: {{.system.processors.models.0}}, RAM: {{.system.memory.system.total}}{% endraw %}"
```

##### OS-Specific Deployment
```yaml
inventory:
  ssh_config:
    user: deploy
    key_file: ~/.ssh/deploy_key
  hosts:
    - name: web1
      address: 192.168.1.10
    - name: web2
      address: 192.168.1.11
      
playbook:
  name: Deploy Application
  facts:
    collectors:
      - name: os_info
        command: facter --json os
        sudo: false
  tasks:
    - name: Install Dependencies (Debian)
      command: apt-get install -y nginx nodejs
      sudo: true
      when: "{% raw %}{{.os_info.os.family}}{% endraw %} == Debian"
      
    - name: Install Dependencies (RedHat)
      command: yum install -y nginx nodejs
      sudo: true
      when: "{% raw %}{{.os_info.os.family}}{% endraw %} == RedHat"
      
    - name: Deploy Application
      command: /usr/local/bin/deploy.sh
      sudo: true
```

### Variable Substitution

SSHOT supports variable substitution in commands, scripts, and file content:
```yaml
# Inventory variables
hosts:
  - name: app1
    vars:
      app_name: myapp
      app_port: "8080"
      app_path: /opt/myapp

# Task using variables
tasks:
  - name: Deploy application
    command: {% raw %}deploy {{.app_name}} --port {{.app_port}} --path {{.app_path}}{% endraw %}
```

### Task Dependencies

Tasks can depend on other tasks:
```yaml
tasks:
  - name: Install dependencies
    command: apt-get install -y build-essential
    
  - name: Build application
    command: make build
    depends_on: [Install dependencies]
    
  - name: Run tests
    command: make test
    depends_on: [Build application]
```

### Group Dependencies

Groups can depend on other groups:
```yaml
groups:
  - name: databases
    order: 1
    hosts: [...]
    
  - name: applications
    order: 2
    depends_on: [databases]
    hosts: [...]
    
  - name: monitoring
    order: 3
    depends_on: [applications]
    hosts: [...]
```

### Task Group Restrictions

Restrict tasks to specific groups:
```yaml
tasks:
  - name: Database Backup
    command: /usr/local/bin/backup-db.sh
    sudo: true
    only_groups: [database]
    
  - name: Web Server Config
    command: /etc/nginx/sites-available/default
    sudo: true
    only_groups: [webserver]
    
  - name: Update All Servers
    command: apt-get update
    sudo: true
    # No only_groups, runs on all hosts
    
  - name: Test Environment Only
    command: /usr/local/bin/test-feature.sh
    skip_groups: [production]
```

### Retries and Error Handling
```yaml
tasks:
  - name: Unreliable task
    command: curl http://api.example.com
    retries: 5                  # Try 5 times
    retry_delay: 2              # 2 seconds between retries
    
  - name: Task that might fail
    command: grep "error" /var/log/app.log
    ignore_error: true          # Continue even if it fails
    
  - name: Task with custom exit codes
    command: grep "pattern" file.txt
    allowed_exit_codes: [0, 1]  # 0=found, 1=not found, both are OK
```

### Timeouts and Progress Indicators
```yaml
tasks:
  - name: Long-running task
    command: backup.sh
    timeout: 300                # 5 minute timeout
```

Run with progress indicators:
```bash
sshot --progress playbook.yml
```

### Local Action, Delegation, and Run Once

These powerful features allow for more sophisticated orchestration patterns:

#### Local Action
```yaml
- name: Run locally
  local_action: echo "Running on the local machine"

- name: Fetch remote logs locally
  local_action: {% raw %}mkdir -p ./logs/{{ .inventory_hostname }}{% endraw %}

- name: Send notification
  local_action: {% raw %}curl -X POST https://api.example.com/notify -d "host={{ .inventory_hostname }}"{% endraw %}
```

`local_action` executes commands on the machine running sshot rather than on the remote hosts. This is useful for:

- Coordinating activities between hosts
- Creating local directories for storing remote data
- Sending notifications about deployment progress
- Interacting with local resources (databases, APIs, files)
- Running commands that require local tools or credentials

For more complex local operations, you can use scripts:
```yaml
- name: Run complex local operations
  local_action: {% raw %}./scripts/local-tasks.sh {{ .inventory_hostname }} {{ .role }}{% endraw %}
```

#### Delegate To
```yaml
- name: Run database backup
  command: pg_dump -U postgres mydb > /tmp/backup.sql
  delegate_to: db-primary

- name: Health check from load balancer
  command: {% raw %}curl -sf http://{{ .inventory_hostname }}:8080/health{% endraw %}
  delegate_to: loadbalancer

- name: Run locally via delegation
  command: {% raw %}./scripts/notify.sh "Deploying to {{ .inventory_hostname }}"{% endraw %}
  delegate_to: localhost
```

The `delegate_to` option runs a command on a specific host instead of the current host in the execution. Key use cases:

- Database operations that should only run on the primary database server
- Load balancer operations (adding/removing hosts)
- Centralized logging or monitoring tasks
- Specialized operations that require specific tools only available on certain hosts

Important notes:
- The delegated host must exist in your inventory
- `delegate_to: localhost` is equivalent to using `local_action`
- Tasks are completely skipped on non-delegated hosts
- Variables from the original host context remain available

#### Run Once
```yaml
- name: Initialize application
  command: ./init-database.sh
  run_once: true

- name: Send deployment notification
  local_action: ./notify.sh "Deployment started"
  run_once: true

- name: Run integration tests
  command: ./run-tests.sh
  run_once: true
  register: test_results
```

The `run_once` flag ensures a task executes on only one host, even when multiple hosts are targeted:

- Perfect for database migrations and schema updates
- Useful for notifications that should happen once per deployment
- Good for integration testing after deployment
- Helpful for initialization tasks that affect the whole system

By default, `run_once` tasks execute on the first host in the inventory. Combine with `delegate_to` to specify which host runs the task.

#### Combining Features

These features are most powerful when combined:
```yaml
- name: Database migration
  command: ./migrate.sh
  delegate_to: db-primary
  run_once: true

- name: Send deployment notification with all hosts
  local_action: {% raw %}./notify-slack.sh "Deploying to {{ groups['web'] | join(', ') }}"{% endraw %}
  run_once: true

- name: Load balancer drain
  command: {% raw %}./lb-control.sh drain {{ .inventory_hostname }}{% endraw %}
  delegate_to: lb-main
  register: lb_status
```

#### Advanced Patterns

**Rolling Deployment with Load Balancer:**
```yaml
tasks:
  - name: Remove from load balancer
    command: {% raw %}./lb-control.sh remove {{ .inventory_hostname }}{% endraw %}
    delegate_to: loadbalancer
    
  - name: Update application
    command: {% raw %}./deploy.sh {{ .version }}{% endraw %}
    
  - name: Verify application health
    command: curl -f http://localhost:8080/health
    retries: 5
    retry_delay: 2
    
  - name: Add back to load balancer
    command: {% raw %}./lb-control.sh add {{ .inventory_hostname }}{% endraw %}
    delegate_to: loadbalancer
```

**Centralized Backup:**
```yaml
tasks:
  - name: Create backup directory
    local_action: {% raw %}mkdir -p ./backups/{{ .timestamp }}/{{ .inventory_hostname }}{% endraw %}
    run_once: true
    vars:
      timestamp: {% raw %}"{{ date +%Y%m%d-%H%M%S }}"{% endraw %}
    
  - name: Backup database
    command: pg_dump -Fc mydb > /tmp/mydb.dump
    
  - name: Fetch backup files
    local_action: {% raw %}scp {{ .user }}@{{ .inventory_hostname }}:/tmp/mydb.dump ./backups/{{ .timestamp }}/{{ .inventory_hostname }}/{% endraw %}
```

**Coordinated Multi-tier Deployment:**
```yaml
tasks:
  - name: Notify deployment start
    local_action: {% raw %}./notify.sh "Starting deployment to {{ .env }}"{% endraw %}
    run_once: true

  - name: Update database schema
    command: ./migrate-db.sh
    delegate_to: db-primary
    run_once: true
    
  - name: Rolling update of application servers
    command: ./deploy-app.sh
    
  - name: Update load balancers
    command: ./update-lb-config.sh
    delegate_to: {% raw %}"{{ item }}"{% endraw %}
    run_once: true
    with_items:
      - lb1
      - lb2
```

## Troubleshooting

### SSH Connection Issues

**Problem: Host key verification failed**
```
host key verification failed for hostname
```

**Solution:**
```bash
ssh-keyscan -H hostname >> ~/.ssh/known_hosts
```

Or disable strict checking in inventory (less secure):
```yaml
ssh_config:
  strict_host_key_check: false
```

**Problem: Authentication failure**

**Check:**
1. SSH key permissions: `chmod 600 ~/.ssh/id_rsa`
2. SSH key path is correct in inventory
3. Correct username and password
4. Try manual SSH: `ssh user@host`

### Task Execution Issues

**Problem: Command not found**

**Solution:**
Use full paths to executables or specify the correct shell.

**Problem: Permission denied**

**Solution:**
Add `sudo: true` to tasks requiring elevated privileges.

**Problem: Timeouts**

**Solution:**
Increase timeout value for long-running tasks:
```yaml
- name: Long task
  command: backup.sh
  timeout: 600  # 10 minutes
```

### Playbook Logic Issues

**Problem: Task skipped unexpectedly**

**Check:**
1. Verify condition syntax in `when` clause
2. Check variable values with verbose mode: `sshot -v playbook.yml`
3. Ensure dependencies are correctly defined

**Problem: Task fails despite retries**

**Solution:**
1. Check retry settings:
```yaml
- name: Flaky task
  command: unreliable.sh
  retries: 10
  retry_delay: 5
```
2. Consider `ignore_error: true` if task is optional
3. Use `allowed_exit_codes` for commands with non-zero success codes

## Getting Help

For more assistance:

- Create an issue on the [GitHub repository](https://github.com/fgouteroux/sshot/issues)
- Check the full source code documentation

## License

Apache License 2.0 - see [LICENSE](LICENSE) file for details.

# sshot Documentation

## Overview
`sshot` is a Go-based tool designed to execute tasks on remote hosts over SSH. It supports both combined and separate configuration files for inventory and playbooks, parallel and sequential execution, dry-run mode, verbose logging, and advanced features like task dependencies, retries, and conditional execution.

## Installation
To use `sshot`, you need Go installed. Clone the repository and build the binary:

```bash
git clone https://github.com/fgouteroux/sshot.git
cd sshot
go build -o sshot
```

## Usage
Run `sshot` with a playbook file and optional flags:

```bash
./sshot [options] <playbook.yml>
```

### Command-Line Options
- `-n, --dry-run`: Run in dry-run mode (simulates execution without making changes).
- `-v, --verbose`: Enable verbose logging for detailed output.
- `-i, --inventory <file>`: Specify a separate inventory file (optional).
- `--progress`: Show progress indicators for long-running tasks.
- `--no-color`: Disable colored output.

### Examples

**Using a combined file (legacy format):**
```bash
./sshot playbook.yml
```

**Using separate inventory and playbook files:**
```bash
./sshot -i inventory.yml playbook.yml
```

**Dry-run with verbose output:**
```bash
./sshot -n -v -i inventory.yml playbook.yml
```

## File Structure

### Option 1: Separate Files (Recommended)

#### Inventory File (`inventory.yml`)
Defines hosts, groups, and SSH configuration:

```yaml
ssh_config:
  user: admin
  key_file: ~/.ssh/id_rsa
  port: 22
  strict_host_key_check: true

hosts:
  - name: server1
    address: 192.168.1.10
    user: user1
    vars:
      env: production

groups:
  - name: webservers
    order: 1
    parallel: true
    hosts:
      - name: web1
        address: 192.168.1.11
      - name: web2
        address: 192.168.1.12
```

#### Playbook File (`playbook.yml`)
Defines the tasks to execute:

```yaml
name: Deploy Application
parallel: false

tasks:
  - name: Install package
    command: apt-get install -y nginx
    sudo: true
    retries: 3
    retry_delay: 5
    
  - name: Copy Config
    copy:
      src: nginx.conf
      dest: /etc/nginx/nginx.conf
      mode: "0644"
```

### Option 2: Combined File (Legacy)

You can still use a single file with both inventory and playbook sections:

```yaml
inventory:
  ssh_config:
    user: admin
    key_file: ~/.ssh/id_rsa
    port: 22
  hosts:
    - name: server1
      address: 192.168.1.10
      user: user1

playbook:
  name: Deploy Application
  tasks:
    - name: Install package
      command: apt-get install -y nginx
      sudo: true
```

## Configuration Reference

### Inventory Section

#### SSH Config (Global Defaults)
- `user`: Default SSH username
- `password`: Default SSH password (not recommended, use keys)
- `key_file`: Path to SSH private key (e.g., `~/.ssh/id_rsa`)
- `key_password`: Password for encrypted SSH key
- `use_agent`: Use SSH agent for authentication (boolean)
- `port`: Default SSH port (default: 22)
- `strict_host_key_check`: Enable host key verification (default: true)

#### Hosts
Individual host definitions:
- `name`: Host identifier (required)
- `address`: IP address or hostname for SSH connection
- `hostname`: Alternative hostname (used if address is not set)
- `port`: SSH port (overrides global default)
- `user`: SSH username (overrides global default)
- `password`: SSH password (overrides global default)
- `key_file`: SSH key path (overrides global default)
- `key_password`: Key password (overrides global default)
- `use_agent`: Use SSH agent (overrides global default)
- `strict_host_key_check`: Host key verification (overrides global default)
- `vars`: Key-value pairs for variable substitution in tasks

#### Groups
Host groups with execution order and dependencies:
- `name`: Group identifier (required)
- `order`: Execution order (lower numbers execute first)
- `parallel`: Execute hosts in parallel (boolean)
- `depends_on`: List of group names that must complete first
- `hosts`: List of host definitions (same structure as inventory hosts)

### Playbook Section

#### Playbook Properties
- `name`: Playbook name (required)
- `parallel`: Execute tasks in parallel across all hosts (boolean)
- `tasks`: List of task definitions (required)

#### Task Properties
- `name`: Task name (required)
- `command`: Shell command to execute
- `shell`: Alternative to command (same functionality)
- `script`: Path to local script file to upload and execute
- `copy`: File copy operation with `src`, `dest`, and `mode`
- `sudo`: Execute with sudo privileges (boolean)
- `when`: Conditional expression (e.g., `{{.var}} == 'value'`)
- `register`: Store task output in a variable
- `ignore_error`: Continue execution on task failure (boolean)
- `vars`: Task-specific variables (merged with host vars)
- `depends_on`: List of task names that must complete first
- `wait_for`: Wait for a condition (`port:8080`, `service:nginx`, `file:/path`, `http://url`)
- `retries`: Number of retry attempts on failure
- `retry_delay`: Seconds to wait between retries
- `timeout`: Task timeout in seconds
- `until_success`: Retry until success (up to 60 attempts)

## Key Features
- **Separate or Combined Files**: Use modular inventory files or combined configuration
- **SSH Authentication**: Supports password, key-based, and SSH agent authentication
- **Task Execution**: Execute commands, scripts, file copies, or wait for conditions
- **Parallel Execution**: Run tasks concurrently across hosts or groups
- **Dry-Run Mode**: Simulate execution without making changes
- **Verbose Logging**: Detailed logs for debugging
- **Task Dependencies**: Enforce order with `depends_on` for tasks and groups
- **Conditional Execution**: Use `when` for conditional task execution
- **Retries and Timeouts**: Handle transient failures with retries and timeouts
- **Variable Substitution**: Use `{{ .var_name }}` in commands, scripts, and file copies
- **Progress Indicators**: Real-time output streaming with `--progress`

## Variable Substitution

Variables can be used in tasks using Go template syntax: `{{ .variable_name }}`

**Variable Sources (in order of precedence):**
1. Task-level `vars`
2. Host-level `vars`
3. Registered variables from previous tasks

**Example:**
```yaml
hosts:
  - name: app-server
    address: 192.168.1.10
    vars:
      app_name: myapp
      app_port: "8080"

tasks:
  - name: Deploy App
    command: deploy {{ .app_name }} --port {{ .app_port }}
    register: deploy_output
    
  - name: Check Deployment
    command: echo {{ .deploy_output }}
```

## Conditional Execution

Use the `when` clause for conditional task execution:

**Syntax:**
- Equality check: `{{ .variable }} == 'value'`
- Variable defined: `variable is defined`

**Example:**
```yaml
tasks:
  - name: Ubuntu Specific Task
    command: apt-get update
    when: "{{ .os }} == ubuntu"
    
  - name: Production Only
    command: deploy-prod.sh
    when: "{{ .env }} == production"
```

## Group Dependencies and Ordering

Groups can have dependencies and execution order:

```yaml
groups:
  - name: databases
    order: 1
    parallel: false
    hosts:
      - name: db1
        address: 192.168.1.20
        
  - name: webservers
    order: 2
    parallel: true
    depends_on: [databases]
    hosts:
      - name: web1
        address: 192.168.1.30
```

## Example Workflows

### Simple Deployment
```bash
# Create inventory.yml and playbook.yml
./sshot -i inventory.yml playbook.yml
```

### Dry-Run Testing
```bash
./sshot -n -v -i inventory.yml playbook.yml
```

### Parallel Execution Across Multiple Groups
```bash
./sshot -i inventory.yml deploy-multiserver.yml
```

## Error Handling
- **Connection Errors**: Retries SSH connections with a 10-second timeout
- **Task Failures**: Supports `ignore_error` to continue on failure
- **Dependencies**: Checks for task and group dependencies before execution
- **Timeouts**: Enforces task timeouts and retries for transient failures

## Contributing
Submit issues or pull requests to the [GitHub repository](https://github.com/fgouteroux/sshot). Ensure code follows Go conventions and includes tests.

## License
© 2025 GitHub, Inc. See the repository for licensing details.
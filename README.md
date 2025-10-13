# sshot Documentation

## Overview
`sshot` is a Go-based tool designed to execute tasks on remote hosts over SSH, using a YAML configuration file (playbook) to define hosts, groups, and tasks. It supports parallel and sequential execution, dry-run mode, verbose logging, and advanced features like task dependencies, retries, and conditional execution.

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
- `--progress`: Show progress indicators for long-running tasks.
- `--no-color`: Disable colored output.

Example:
```bash
./sshot -p playbook.yml
```

## Playbook Structure
The playbook is a YAML file with two main sections: `inventory` and `playbook`.

### Inventory
Defines hosts and groups for task execution.

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
  groups:
    - name: webservers
      order: 1
      hosts:
        - name: web1
          address: 192.168.1.11
      parallel: true
      depends_on: [dbservers]
```

- `ssh_config`: Default SSH settings (user, password, key_file, key_password, use_agent, port).
- `hosts`: List of individual hosts with properties like `name`, `address`, `hostname`, `port`, `user`, `password`, `key_file`, `key_password`, `use_agent`, and `vars`.
- `groups`: List of host groups with `name`, `hosts`, `parallel`, `order`, and `depends_on`.

### Playbook
Defines tasks to execute on hosts.

```yaml
playbook:
  name: Deploy Application
  parallel: false
  tasks:
    - name: Install package
      command: apt-get install -y nginx
      sudo: true
      retries: 3
      retry_delay: 5
      when: ansible_os == 'ubuntu'
      register: install_result
```

- `name`: Playbook name.
- `parallel`: Whether tasks run in parallel across hosts.
- `tasks`: List of tasks with properties:
  - `name`: Task name.
  - `command`: Shell command to execute.
  - `script`: Path to a script to run.
  - `copy`: File copy task with `src`, `dest`, and `mode`.
  - `shell`: Shell command (similar to `command`).
  - `sudo`: Run with sudo (default: false).
  - `when`: Condition for task execution (e.g., variable checks).
  - `register`: Store task output in a variable.
  - `ignore_error`: Continue on task failure (default: false).
  - `depends_on`: List of task dependencies.
  - `wait_for`: Wait for a condition (e.g., `port:8080`, `service:nginx`).
  - `retries`: Number of retries on failure.
  - `retry_delay`: Delay between retries (seconds).
  - `timeout`: Task timeout (seconds).
  - `until_success`: Retry until success (up to 60 retries).

## Key Features
- **SSH Authentication**: Supports password, key-based, and SSH agent authentication.
- **Task Execution**: Execute commands, scripts, file copies, or wait for conditions.
- **Parallel Execution**: Run tasks concurrently across hosts or groups.
- **Dry-Run Mode**: Simulate execution without making changes.
- **Verbose Logging**: Detailed logs for debugging.
- **Task Dependencies**: Enforce order with `depends_on` for tasks and groups.
- **Conditional Execution**: Use `when` for conditional task execution.
- **Retries and Timeouts**: Handle transient failures with retries and timeouts.
- **Variable Substitution**: Use `{{ var_name }}` in commands, scripts, and file copies.
- **Progress Indicators**: Real-time output streaming with `--progress`.

## Code Structure
The main components are:

- **Config**: Parses the YAML playbook into `Inventory` and `Playbook` structs.
- **Executor**: Manages SSH connections and task execution for a host.
- **HostResult**: Stores execution results (success, error, output).
- **ExecutionOptions**: Holds runtime flags (dry-run, verbose, parallel, etc.).
- **Task Execution**: Supports commands, scripts, file copies, and wait conditions with retries and timeouts.
- **Variable Management**: Substitutes variables in tasks using a template engine.
- **Colorized Output**: Uses ANSI colors for readable console output (disable with `--no-color`).

## Example Playbook
```yaml
inventory:
  hosts:
    - name: server1
      address: 192.168.1.10
      user: admin
      key_file: ~/.ssh/id_rsa
playbook:
  name: Setup Web Server
  tasks:
    - name: Install Nginx
      command: apt-get install -y nginx
      sudo: true
    - name: Copy Config
      copy:
        src: nginx.conf
        dest: /etc/nginx/nginx.conf
        mode: "0644"
    - name: Restart Nginx
      command: systemctl restart nginx
      sudo: true
      wait_for: port:80
```

## Running the Playbook
```bash
./sshot -v playbook.yml
```

This will:
1. Connect to `server1` via SSH.
2. Install Nginx with sudo.
3. Copy `nginx.conf` to the remote server.
4. Restart Nginx and wait for port 80 to be available.

## Error Handling
- **Connection Errors**: Retries SSH connections with a 10-second timeout.
- **Task Failures**: Supports `ignore_error` to continue on failure.
- **Dependencies**: Checks for task and group dependencies before execution.
- **Timeouts**: Enforces task timeouts and retries for transient failures.

## Limitations
- Requires a valid SSH configuration (password, key, or agent).
- No support for advanced SSH features like bastion hosts.
- Limited `wait_for` types: `port`, `service`, `file`, `http`.
- YAML parsing errors require manual playbook correction.

## Contributing
Submit issues or pull requests to the [GitHub repository](https://github.com/fgouteroux/sshot). Ensure code follows Go conventions and includes tests.

## License
© 2025 GitHub, Inc. See the repository for licensing details.
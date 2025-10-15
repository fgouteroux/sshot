# sshot - SSH Orchestrator Tool

[![Go Report Card](https://goreportcard.com/badge/github.com/fgouteroux/sshot)](https://goreportcard.com/report/github.com/fgouteroux/sshot)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/fgouteroux/sshot)](https://github.com/fgouteroux/sshot/releases)

> Orchestrate SSH operations with ease

**sshot** (SSH Orchestrator Tool) is a lightweight, Ansible-inspired tool for sysadmins who need straightforward SSH orchestration without Python dependency headaches. Built with Go for portability and simplicity, it uses familiar YAML playbooks—perfect for daily administrative tasks.

## Why sshot?

If you're a sysadmin who loves Ansible's YAML approach but sometimes finds Python dependencies challenging, sshot might be for you.

**sshot is NOT a replacement for Ansible** - it doesn't try to be. Ansible is a comprehensive automation platform with an extensive ecosystem. sshot is simply a focused helper tool for sysadmins who need straightforward SSH orchestration.

### Key Benefits

- 🪶 **No Python headaches** - Single Go binary, no dependencies, no virtualenvs, no pip issues
- 🎯 **Sysadmin-focused** - Built for daily SSH tasks, not enterprise-wide automation
- ⚡ **Portable** - Copy one binary, run anywhere (Linux, macOS, even on edge devices)
- 📝 **Familiar syntax** - If you know Ansible YAML, you already know sshot
- 🚀 **Fast** - Go's performance for quick task execution
 
## Installation

### From Source
```bash
go install github.com/fgouteroux/sshot@latest
```

### From Release
```bash
# Download from GitHub releases
wget https://github.com/fgouteroux/sshot/releases/latest/download/sshot_Linux_x86_64.tar.gz
tar xzf sshot_Linux_x86_64.tar.gz
sudo mv sshot /usr/local/bin/
```

### Build Locally
```bash
git clone https://github.com/fgouteroux/sshot.git
cd sshot
go build -o sshot
```

## Quick Start

### 1. Create an inventory file
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

### 2. Create a playbook
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

### 3. Run it
```bash
sshot -i inventory.yml playbook.yml
```

## Usage

```bash
sshot [options] <playbook.yml>
```

### Options
- `-i, --inventory <file>` - Inventory file (supports separate files)
- `-n, --dry-run` - Run in dry-run mode (simulate without executing)
- `-v, --verbose` - Enable verbose logging
- `--progress` - Show progress indicators
- `--no-color` - Disable colored output

### Examples

**Basic execution:**
```bash
sshot playbook.yml
```

**With separate inventory:**
```bash
sshot -i inventory.yml playbook.yml
```

**Dry-run mode:**
```bash
sshot -n -v -i inventory.yml playbook.yml
```

**With progress indicators:**
```bash
sshot --progress -i inventory.yml playbook.yml
```

## Features

### Ansible-Inspired, Sysadmin-Focused

sshot borrows Ansible's excellent design philosophy but focuses specifically on sysadmin needs:
- ✅ YAML playbooks for configuration
- ✅ Inventory files for host management
- ✅ Task execution with dependencies
- ✅ Conditional task execution (`when`)
- ✅ Variable substitution
- ✅ Parallel and sequential execution
- ✅ Group-based orchestration

### Core Capabilities

#### 1. Flexible Execution Modes
- **Parallel execution** across multiple hosts
- **Sequential execution** with ordered groups
- **Group dependencies** for complex workflows

#### 2. Task Types
- **Commands** - Execute shell commands
- **Scripts** - Upload and run local scripts
- **File copy** - Copy files with permissions
- **Wait conditions** - Wait for ports, services, files, HTTP endpoints

#### 3. Advanced Features
- **Retries** - Automatic retry with configurable delays
- **Timeouts** - Task-level timeout control
- **Conditionals** - Execute tasks based on variables
- **Dependencies** - Define task execution order
- **Variable substitution** - Use variables in commands and files
- **Register output** - Capture and reuse task output

#### 4. Authentication
- SSH key-based authentication
- Password authentication
- SSH agent support
- Per-host authentication override

## Configuration

### Inventory Structure

#### Global SSH Configuration
```yaml
ssh_config:
  user: admin
  key_file: ~/.ssh/id_rsa
  port: 22
  strict_host_key_check: true  # Set to false to disable verification
```

#### Hosts
```yaml
hosts:
  - name: server1
    address: 192.168.1.10
    user: deploy        # Override global user
    vars:
      env: production
      app_port: "8080"
```

#### Groups with Dependencies
```yaml
groups:
  - name: databases
    order: 1
    parallel: false
    hosts:
      - name: db1
        address: 192.168.1.20
        vars:
          role: master
      - name: db2
        address: 192.168.1.21
        vars:
          role: slave
          
  - name: webservers
    order: 2
    parallel: true
    depends_on: [databases]  # Wait for databases group
    hosts:
      - name: web1
        address: 192.168.1.30
      - name: web2
        address: 192.168.1.31
```

### Playbook Structure

```yaml
name: Multi-tier Deployment
parallel: false  # Global parallel setting

tasks:
  - name: Check connectivity
    command: echo "Connected to {{ .hostname }}"
    
  - name: Install packages
    command: apt-get install -y nginx mysql-client
    sudo: true
    retries: 3
    retry_delay: 5
    
  - name: Copy configuration
    copy:
      src: ./nginx.conf
      dest: /etc/nginx/nginx.conf
      mode: "0644"
    sudo: true
    
  - name: Start service
    command: systemctl start nginx
    sudo: true
    wait_for: port:80
    
  - name: Health check
    command: curl -f http://localhost/health
    retries: 5
    retry_delay: 2
    until_success: true
    
  - name: Production only task
    command: deploy-prod.sh
    when: "{{ .env }} == 'production'"
    register: deploy_output
```

## Task Reference

### Basic Task
```yaml
- name: Execute command
  command: echo "hello world"
```

### Task with Sudo
```yaml
- name: Install package
  command: apt-get install -y nginx
  sudo: true
```

### Task with Variables
```yaml
- name: Deploy application
  command: deploy {{ .app_name }} --port {{ .app_port }}
```

### Task with Conditionals
```yaml
- name: Ubuntu specific
  command: apt-get update
  when: "{{ .os }} == 'ubuntu'"
```

### Task with Retries
```yaml
- name: Download artifact
  command: wget https://example.com/artifact.tar.gz
  retries: 3
  retry_delay: 5
```

### Task with Dependencies
```yaml
- name: Build application
  command: make build
  depends_on: [Install Dependencies, Clone Repository]
```

### Task with Allowed Exit Codes
```yaml
- name: Search for pattern
  command: grep "error" /var/log/app.log
  allowed_exit_codes: [0, 1]  # 0 = found, 1 = not found
  register: search_result

- name: Compare files
  command: diff file1.txt file2.txt
  allowed_exit_codes: [0, 1]  # 0 = identical, 1 = different

- name: Custom script
  command: ./check_status.sh
  allowed_exit_codes: [0, 2, 3]  # Multiple allowed codes
```

This is useful for commands like `grep`, `diff`, or custom scripts where
non-zero exit codes have specific meanings that should still be considered
successful. If a command exits with a code in the allowed list, it will
be treated as successful and won't trigger retries or fail the playbook.

### File Copy Task
```yaml
- name: Copy config
  copy:
    src: local/config.yml
    dest: /etc/app/config.yml
    mode: "0644"
  sudo: true
```

### Script Execution
```yaml
- name: Run setup script
  script: ./scripts/setup.sh
  sudo: true
```

### Wait for Condition
```yaml
- name: Wait for database
  wait_for: port:5432

- name: Wait for service
  wait_for: service:postgresql

- name: Wait for file
  wait_for: file:/var/run/app.pid

- name: Wait for HTTP
  wait_for: http://localhost:8080/health
```

## Examples

### Simple Deployment
```yaml
# inventory.yml
ssh_config:
  user: deploy
  key_file: ~/.ssh/deploy_key

hosts:
  - name: prod-server
    address: production.example.com

# playbook.yml
name: Deploy Website
tasks:
  - name: Pull latest code
    command: git pull origin main
    
  - name: Install dependencies
    command: npm install
    
  - name: Build application
    command: npm run build
    
  - name: Restart service
    command: systemctl restart app
    sudo: true
```

### Multi-tier Application
```yaml
# inventory.yml
ssh_config:
  user: admin
  key_file: ~/.ssh/id_rsa

groups:
  - name: database
    order: 1
    hosts:
      - name: db-primary
        address: 10.0.1.10
        vars:
          role: primary
      - name: db-replica
        address: 10.0.1.11
        vars:
          role: replica
          
  - name: application
    order: 2
    parallel: true
    depends_on: [database]
    hosts:
      - name: app1
        address: 10.0.2.10
      - name: app2
        address: 10.0.2.11
        
  - name: loadbalancer
    order: 3
    depends_on: [application]
    hosts:
      - name: lb1
        address: 10.0.3.10

# playbook.yml
name: Deploy Multi-tier Application
tasks:
  - name: Stop application
    command: systemctl stop myapp
    ignore_error: true
    
  - name: Backup database
    command: pg_dump mydb > /backup/mydb.sql
    when: "{{ .role }} == 'primary'"
    sudo: true
    
  - name: Update application
    command: deploy.sh --version {{ .version }}
    vars:
      version: "2.0.0"
    retries: 3
    
  - name: Start application
    command: systemctl start myapp
    sudo: true
    
  - name: Wait for service
    wait_for: port:8080
    
  - name: Health check
    command: curl -f http://localhost:8080/health
    retries: 10
    retry_delay: 3
```

## Troubleshooting

### Host Key Verification Failed

**Error:**
```
host key verification failed for hostname: knownhosts: key is unknown
To add this host, run: ssh-keyscan -H hostname >> /home/user/.ssh/known_hosts
```

**Solution 1: Add the host key**
```bash
ssh-keyscan -H hostname >> ~/.ssh/known_hosts
```

**Solution 2: Disable strict checking (not recommended for production)**
```yaml
ssh_config:
  strict_host_key_check: false
```

### Authentication Failed

**Check:**
1. SSH key permissions: `chmod 600 ~/.ssh/id_rsa`
2. SSH key path is correct in inventory
3. User has SSH access to the host
4. Try manual SSH: `ssh user@host`

### Connection Timeout

**Solutions:**
- Check host is reachable: `ping hostname`
- Verify port is correct (default: 22)
- Check firewall rules
- Verify SSH service is running: `systemctl status sshd`

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

### Development Setup

```bash
# Clone repository
git clone https://github.com/fgouteroux/sshot.git
cd sshot

# Install dependencies
go mod download

# Run tests
make test

# Build
make build

# Run linter
make lint
```

### Running Tests

```bash
# All tests
go test -v ./...

# With coverage
go test -cover ./...

# Specific package
go test -v ./pkg/config/...
```

## License

Apache License 2.0 - see [LICENSE](LICENSE) file for details.

## Author

François Gouteroux

## Acknowledgments

- Inspired by [Ansible](https://www.ansible.com/) - for pioneering YAML-based automation
- Built with [Go](https://golang.org/) - for performance and simplicity
- Uses [golang.org/x/crypto/ssh](https://pkg.go.dev/golang.org/x/crypto/ssh) - for SSH connectivity

---

**sshot** - SSH Orchestrator Tool | [GitHub](https://github.com/fgouteroux/sshot) | [Documentation](https://github.com/fgouteroux/sshot/wiki)
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

func NewExecutor(host Host) (*Executor, error) {
	if execOptions.Verbose {
		log.Printf("[VERBOSE] Connecting to host: %s", host.Name)
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
		vars := make(map[string]string)
		if host.Vars != nil {
			for k, v := range host.Vars {
				vars[k] = v
			}
		}

		return &Executor{
			host:           host,
			client:         nil,
			variables:      vars,
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

func getHostKeyCallback(strictHostKeyCheck *bool) (ssh.HostKeyCallback, error) {
	// Determine the actual value to use (default to true if nil)
	strict := true
	if strictHostKeyCheck != nil {
		strict = *strictHostKeyCheck
	}

	// If strict host key checking is disabled, use insecure callback
	// This is useful for testing environments but should be avoided in production
	if !strict {
		if execOptions.Verbose {
			log.Printf("[VERBOSE] WARNING: Host key verification is disabled (strict_host_key_check: false)")
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

		if execOptions.Verbose {
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

package sync

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Hosts that should be filtered out from SSH config discovery.
// These are typically code hosting services, not personal machines.
var skipHosts = map[string]bool{
	"github.com":     true,
	"gitlab.com":     true,
	"bitbucket.org":  true,
	"codeberg.org":   true,
	"sr.ht":          true,
	"ssh.github.com": true,
}

// DiscoverFromSSHConfig parses ~/.ssh/config and returns discovered machines.
// It filters out:
//   - Known code hosting services (github, gitlab, etc.)
//   - Wildcard hosts (Host *)
//   - Hosts with ProxyJump (complex setups)
func DiscoverFromSSHConfig() ([]*Machine, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(homeDir, ".ssh", "config")
	return parseSSHConfig(configPath)
}

// parseSSHConfig parses an SSH config file and returns machines.
func parseSSHConfig(path string) ([]*Machine, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No SSH config file
		}
		return nil, err
	}
	defer file.Close()

	var machines []*Machine
	var current *sshHost
	var currentHasProxyJump bool

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse key-value pairs
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			// Try with tab separator
			parts = strings.SplitN(line, "\t", 2)
			if len(parts) < 2 {
				continue
			}
		}

		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])

		switch key {
		case "host":
			// Save previous host if valid
			if current != nil && !currentHasProxyJump {
				if m := current.toMachine(); m != nil {
					machines = append(machines, m)
				}
			}

			// Start new host
			// Skip wildcards
			if strings.Contains(value, "*") || strings.Contains(value, "?") {
				current = nil
				currentHasProxyJump = false
				continue
			}

			// Skip known hosts
			if skipHosts[strings.ToLower(value)] {
				current = nil
				currentHasProxyJump = false
				continue
			}

			current = &sshHost{
				name: value,
			}
			currentHasProxyJump = false

		case "hostname":
			if current != nil {
				current.hostname = value
			}

		case "port":
			if current != nil {
				current.port = value
			}

		case "user":
			if current != nil {
				current.user = value
			}

		case "identityfile":
			if current != nil {
				current.identityFile = value
			}

		case "proxyjump", "proxycommand":
			// Skip hosts with proxy configurations (complex setups)
			currentHasProxyJump = true
		}
	}

	// Save last host
	if current != nil && !currentHasProxyJump {
		if m := current.toMachine(); m != nil {
			machines = append(machines, m)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return machines, nil
}

// sshHost is an intermediate representation of an SSH config host.
type sshHost struct {
	name         string
	hostname     string
	port         string
	user         string
	identityFile string
}

// toMachine converts an SSH host to a Machine.
// Returns nil if the host should be skipped.
func (h *sshHost) toMachine() *Machine {
	if h.name == "" {
		return nil
	}

	// Determine address
	address := h.hostname
	if address == "" {
		address = h.name
	}

	// Skip if the hostname is a known service
	if skipHosts[strings.ToLower(address)] {
		return nil
	}

	m := NewMachine(h.name, address)
	m.Source = SourceSSHConfig

	// Parse port
	if h.port != "" {
		host, port, _ := ParseAddress(":" + h.port)
		if host == "" && port != 0 {
			m.Port = port
		}
	}

	// Set user
	if h.user != "" {
		m.SSHUser = h.user
	}

	// Set identity file
	if h.identityFile != "" {
		m.SSHKeyPath = expandPath(h.identityFile)
	}

	return m
}

// expandPath expands ~ to the user's home directory.
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if homeDir, err := os.UserHomeDir(); err == nil {
			return filepath.Join(homeDir, path[2:])
		}
	}
	return path
}

// MergeDiscoveredMachines merges newly discovered machines with existing ones.
// Existing machines (by name) are preserved; new ones are added.
// Returns the merged list.
func MergeDiscoveredMachines(existing, discovered []*Machine) []*Machine {
	// Build a map of existing machine names
	existingNames := make(map[string]bool)
	for _, m := range existing {
		existingNames[strings.ToLower(m.Name)] = true
	}

	// Start with existing machines
	result := make([]*Machine, len(existing))
	copy(result, existing)

	// Add new discovered machines
	for _, m := range discovered {
		if !existingNames[strings.ToLower(m.Name)] {
			result = append(result, m)
		}
	}

	return result
}

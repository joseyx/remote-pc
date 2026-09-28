package vpn

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Status struct {
	Installed bool   `json:"installed"`
	Connected bool   `json:"connected"`
	State     string `json:"state"` // "Connected", "Disconnected", "Connecting", "Disconnecting"
	Location  string `json:"location"`
	IP        string `json:"ip"`
	Protocol  string `json:"protocol"`
	Firewall  string `json:"firewall"`
	Error     string `json:"error,omitempty"`
}

type ConnectOptions struct {
	Location string `json:"location,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Port     string `json:"port,omitempty"`
}

type LocationItem struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Group string `json:"group"`
}

type ProtocolItem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Ports       []string `json:"ports"`
}

type VpnOptions struct {
	Locations []LocationItem `json:"locations"`
	Protocols []ProtocolItem `json:"protocols"`
}

var AvailableLocations = []LocationItem{
	{ID: "best", Name: "Best Location (Auto)", Group: "Auto"},
	{ID: "Miami", Name: "Miami, FL", Group: "US East"},
	{ID: "New York", Name: "New York, NY", Group: "US East"},
	{ID: "Atlanta", Name: "Atlanta, GA", Group: "US East"},
	{ID: "Dallas", Name: "Dallas, TX", Group: "US Central"},
	{ID: "Chicago", Name: "Chicago, IL", Group: "US Central"},
	{ID: "Los Angeles", Name: "Los Angeles, CA", Group: "US West"},
	{ID: "Seattle", Name: "Seattle, WA", Group: "US West"},
	{ID: "Toronto", Name: "Toronto", Group: "Canada"},
	{ID: "London", Name: "London", Group: "Europe"},
	{ID: "Frankfurt", Name: "Frankfurt", Group: "Europe"},
	{ID: "Paris", Name: "Paris", Group: "Europe"},
	{ID: "Amsterdam", Name: "Amsterdam", Group: "Europe"},
	{ID: "Zurich", Name: "Zurich", Group: "Europe"},
	{ID: "Tokyo", Name: "Tokyo", Group: "Asia"},
}

var AvailableProtocols = []ProtocolItem{
	{ID: "wireguard", Name: "WireGuard", Description: "Fastest, lowest latency (Recommended)", Ports: []string{"443", "80", "53", "1194", "65142"}},
	{ID: "stealth", Name: "Stealth", Description: "Obfuscated TLS / Anti-DPI (Bypasses geo-blocks)", Ports: []string{"443", "8080", "8443"}},
	{ID: "udp", Name: "OpenVPN UDP", Description: "Standard OpenVPN protocol", Ports: []string{"443", "80", "53", "1194"}},
	{ID: "tcp", Name: "OpenVPN TCP", Description: "Reliable over restricted networks", Ports: []string{"443", "80", "53"}},
	{ID: "wstunnel", Name: "WSTunnel", Description: "WebSocket tunnel encapsulation", Ports: []string{"443", "8080"}},
	{ID: "ikev2", Name: "IKEv2", Description: "Fast reconnection", Ports: []string{"500"}},
}

type Manager struct {
	mu         sync.RWMutex
	cliPath    string
	lastStatus Status
	lastFetch  time.Time
	isBusy     bool
}

func NewManager() *Manager {
	cli := findWindscribeCLI()
	m := &Manager{
		cliPath: cli,
	}
	m.Refresh()
	return m
}

func findWindscribeCLI() string {
	candidates := []string{
		`C:\Program Files\Windscribe\windscribe-cli.exe`,
		`C:\Program Files (x86)\Windscribe\windscribe-cli.exe`,
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	if p, err := exec.LookPath("windscribe-cli.exe"); err == nil {
		return p
	}
	if p, err := exec.LookPath("windscribe-cli"); err == nil {
		return p
	}

	if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
		p := filepath.Join(localApp, "Programs", "Windscribe", "windscribe-cli.exe")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return ""
}

// runQuick runs a windscribe-cli command and waits for process exit without waiting for pipe EOF
func (m *Manager) runQuick(timeout time.Duration, args ...string) ([]byte, error) {
	if m.cliPath == "" {
		return nil, exec.ErrNotFound
	}

	cmd := exec.Command(m.cliPath, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	done := make(chan error, 1)
	go func() {
		_, err := cmd.Process.Wait()
		done <- err
	}()

	select {
	case err := <-done:
		return stdout.Bytes(), err
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		return stdout.Bytes(), nil
	}
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	if time.Since(m.lastFetch) < 3*time.Second && !m.isBusy {
		st := m.lastStatus
		m.mu.RUnlock()
		return st
	}
	m.mu.RUnlock()

	return m.Refresh()
}

func (m *Manager) Refresh() Status {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cliPath == "" {
		m.cliPath = findWindscribeCLI()
	}

	if m.cliPath == "" {
		m.lastStatus = Status{
			Installed: false,
			State:     "Not Installed",
		}
		m.lastFetch = time.Now()
		return m.lastStatus
	}

	out, err := m.runQuick(2*time.Second, "status")
	if err != nil && len(out) == 0 {
		if m.isBusy {
			return m.lastStatus
		}
		m.lastStatus = Status{
			Installed: true,
			State:     "Error",
			Error:     err.Error(),
		}
		m.lastFetch = time.Now()
		return m.lastStatus
	}

	st := Status{
		Installed: true,
		State:     "Disconnected",
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch strings.ToLower(key) {
		case "connect state":
			if strings.HasPrefix(strings.ToLower(val), "connected") {
				st.Connected = true
				st.State = "Connected"
				sub := strings.SplitN(val, ":", 2)
				if len(sub) > 1 {
					st.Location = strings.TrimSpace(sub[1])
				}
			} else if strings.Contains(strings.ToLower(val), "connecting") || strings.Contains(strings.ToLower(val), "progress") {
				st.State = "Connecting"
			} else {
				st.State = "Disconnected"
			}
		case "vpn ip":
			st.IP = val
		case "public ip":
			if st.IP == "" {
				st.IP = val
			}
		case "protocol":
			st.Protocol = val
		case "firewall state":
			st.Firewall = val
		}
	}

	m.lastStatus = st
	m.lastFetch = time.Now()
	return st
}

func (m *Manager) GetOptions() VpnOptions {
	return VpnOptions{
		Locations: AvailableLocations,
		Protocols: AvailableProtocols,
	}
}

func (m *Manager) Connect(opts ...ConnectOptions) (Status, error) {
	m.mu.Lock()
	if m.cliPath == "" {
		m.mu.Unlock()
		return Status{Installed: false}, exec.ErrNotFound
	}
	m.isBusy = true
	m.lastStatus.State = "Connecting"
	current := m.lastStatus
	m.mu.Unlock()

	var opt ConnectOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	go func() {
		defer func() {
			m.mu.Lock()
			m.isBusy = false
			m.mu.Unlock()
			m.Refresh()
		}()

		args := []string{"connect", "-n"}
		loc := strings.TrimSpace(opt.Location)
		if loc != "" {
			args = append(args, loc)
		}
		proto := strings.ToLower(strings.TrimSpace(opt.Protocol))
		if proto != "" {
			port := strings.TrimSpace(opt.Port)
			if port != "" {
				proto += ":" + port
			}
			args = append(args, proto)
		}

		// Run non-blocking connect (returns almost immediately with -n)
		_, _ = m.runQuick(10*time.Second, args...)
	}()

	return current, nil
}

func (m *Manager) Disconnect() (Status, error) {
	m.mu.Lock()
	if m.cliPath == "" {
		m.mu.Unlock()
		return Status{Installed: false}, exec.ErrNotFound
	}
	m.isBusy = true
	m.lastStatus.State = "Disconnecting"
	current := m.lastStatus
	m.mu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			m.isBusy = false
			m.mu.Unlock()
			m.Refresh()
		}()

		_, _ = m.runQuick(10*time.Second, "disconnect")
	}()

	return current, nil
}

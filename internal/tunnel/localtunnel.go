package tunnel

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SimpleTunnelClient creates a simple HTTP tunnel using available tools
type SimpleTunnelClient struct {
	localAddr      string
	subdomain      string
	tunnelURL      string
	cmd            *exec.Cmd
	cancel         context.CancelFunc
	urlFile        string
	connected      bool
	onStatusChange func(connected bool, url string)
}

// NewSimpleTunnelClient creates a new tunnel client
func NewSimpleTunnelClient(localAddr, subdomain string) *SimpleTunnelClient {
	home, _ := os.UserHomeDir()
	urlFile := filepath.Join(home, ".config", "modeltunnel", "tunnel.url")

	return &SimpleTunnelClient{
		localAddr: localAddr,
		subdomain: subdomain,
		urlFile:   urlFile,
	}
}

// SetStatusCallback sets a callback for connection status changes
func (c *SimpleTunnelClient) SetStatusCallback(cb func(connected bool, url string)) {
	c.onStatusChange = cb
}

// Start starts the tunnel and returns the public URL
func (c *SimpleTunnelClient) Start() (string, error) {
	// Try ngrok first (most reliable)
	if url, err := c.tryNgrok(); err == nil {
		return url, nil
	}

	// Try localtunnel npm package
	if url, err := c.tryLocalTunnel(); err == nil {
		return url, nil
	}

	// If both fail, return error with instructions
	return "", fmt.Errorf("no tunnel tool available")
}

// tryNgrok tries to use ngrok if available
func (c *SimpleTunnelClient) tryNgrok() (string, error) {
	ngrokPath, err := exec.LookPath("ngrok")
	if err != nil {
		return "", fmt.Errorf("ngrok not found")
	}

	port := strings.Split(c.localAddr, ":")[1]

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	args := []string{"http", port}
	if c.subdomain != "" {
		args = append(args, "--subdomain", c.subdomain)
	}

	c.cmd = exec.CommandContext(ctx, ngrokPath, args...)

	// Capture both stdout and stderr (ngrok prints to stderr)
	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		cancel()
		return "", err
	}

	stderr, err := c.cmd.StderrPipe()
	if err != nil {
		cancel()
		return "", err
	}

	if err := c.cmd.Start(); err != nil {
		cancel()
		return "", err
	}

	// Parse ngrok output to find URL (check both stdout and stderr)
	urlChan := make(chan string, 1)

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println("ngrok-stdout:", line)
			if strings.Contains(line, "https://") && strings.Contains(line, ".ngrok") {
				parts := strings.Fields(line)
				for _, part := range parts {
					if strings.HasPrefix(part, "https://") {
						urlChan <- part
						return
					}
				}
			}
		}
	}()

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println("ngrok-stderr:", line)
			if strings.Contains(line, "Forwarding") && strings.Contains(line, "https://") {
				parts := strings.Fields(line)
				for _, part := range parts {
					if strings.HasPrefix(part, "https://") {
						urlChan <- part
						return
					}
				}
			}
		}
	}()

	// Wait for URL or timeout (30 seconds for slow connections)
	select {
	case url := <-urlChan:
		c.tunnelURL = url
		c.connected = true
		if c.onStatusChange != nil {
			c.onStatusChange(true, url)
		}
		c.saveURL()
		return url, nil
	case <-time.After(30 * time.Second):
		cancel()
		return "", fmt.Errorf("timeout waiting for ngrok")
	}
}

// tryLocalTunnel tries to use localtunnel npm package
func (c *SimpleTunnelClient) tryLocalTunnel() (string, error) {
	// Check if lt is available
	ltPath, err := exec.LookPath("lt")
	if err != nil {
		// Try npx
		ltPath = "npx"
	}

	port := strings.Split(c.localAddr, ":")[1]

	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	var args []string
	if ltPath == "npx" {
		args = []string{"localtunnel", "--port", port}
	} else {
		args = []string{"--port", port}
	}

	if c.subdomain != "" {
		args = append(args, "--subdomain", c.subdomain)
	}

	c.cmd = exec.CommandContext(ctx, ltPath, args...)

	// Capture both stdout and stderr (localtunnel prints URL to stderr)
	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		cancel()
		return "", err
	}

	stderr, err := c.cmd.StderrPipe()
	if err != nil {
		cancel()
		return "", err
	}

	if err := c.cmd.Start(); err != nil {
		cancel()
		return "", err
	}

	// Parse both stdout and stderr to find URL
	urlChan := make(chan string, 1)

	// Scan stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println("lt-stdout:", line)

			url := extractURL(line)
			if url != "" {
				urlChan <- url
				return
			}
		}
	}()

	// Scan stderr (localtunnel prints URL here)
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println("lt-stderr:", line)

			// Extract URL from the line
			url := extractURL(line)
			if url != "" {
				urlChan <- url
				return
			}
		}
	}()

	// Wait for URL or timeout (30 seconds for slow connections)
	select {
	case url := <-urlChan:
		c.tunnelURL = url
		c.connected = true
		if c.onStatusChange != nil {
			c.onStatusChange(true, url)
		}
		c.saveURL()
		return url, nil
	case <-time.After(30 * time.Second):
		cancel()
		return "", fmt.Errorf("timeout waiting for localtunnel")
	}
}

// extractURL extracts a URL containing loca.lt or ngrok from a string
func extractURL(line string) string {
	// Look for http:// or https:// URLs
	if idx := strings.Index(line, "http://"); idx != -1 {
		url := line[idx:]
		// Find the end of the URL (space or end of line)
		if endIdx := strings.IndexFunc(url, func(r rune) bool { return r == ' ' || r == '\t' }); endIdx != -1 {
			url = url[:endIdx]
		}
		// Clean up any trailing punctuation
		url = strings.TrimRight(url, ".,;:!?'")
		if strings.Contains(url, "loca.lt") || strings.Contains(url, "ngrok") || strings.Contains(url, "localtunnel") {
			return url
		}
	}
	if idx := strings.Index(line, "https://"); idx != -1 {
		url := line[idx:]
		// Find the end of the URL
		if endIdx := strings.IndexFunc(url, func(r rune) bool { return r == ' ' || r == '\t' }); endIdx != -1 {
			url = url[:endIdx]
		}
		// Clean up any trailing punctuation
		url = strings.TrimRight(url, ".,;:!?'")
		if strings.Contains(url, "loca.lt") || strings.Contains(url, "ngrok") || strings.Contains(url, "localtunnel") {
			return url
		}
	}
	return ""
}

// Stop stops the tunnel
func (c *SimpleTunnelClient) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
	c.connected = false
	if c.onStatusChange != nil {
		c.onStatusChange(false, "")
	}
	os.Remove(c.urlFile)
}

// URL returns the tunnel URL
func (c *SimpleTunnelClient) URL() string {
	return c.tunnelURL
}

// IsConnected returns whether the tunnel is currently connected
func (c *SimpleTunnelClient) IsConnected() bool {
	return c.connected
}

// saveURL saves the tunnel URL to a file
func (c *SimpleTunnelClient) saveURL() error {
	dir := filepath.Dir(c.urlFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(c.urlFile, []byte(c.tunnelURL), 0600)
}

// GetLocalIP returns the local IP address
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return "127.0.0.1"
}

// NewLocalTunnelClient is an alias for NewSimpleTunnelClient for backward compatibility
var NewLocalTunnelClient = NewSimpleTunnelClient

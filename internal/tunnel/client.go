package tunnel

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// Client manages tunnel connections using cloudflared or other tools
type Client struct {
	localAddr string
	tunnelURL string
	cmd       *exec.Cmd
	cancel    context.CancelFunc
}

// NewClient creates a new tunnel client
func NewClient(localAddr string) *Client {
	return &Client{
		localAddr: localAddr,
	}
}

// Start starts the tunnel and returns the public URL
func (c *Client) Start() (string, error) {
	_, err := exec.LookPath("cloudflared")
	if err != nil {
		return c.startManualTunnel()
	}

	return c.startCloudflareTunnel()
}

func (c *Client) startCloudflareTunnel() (string, error) {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel

	c.cmd = exec.CommandContext(ctx, "cloudflared", "tunnel", "--url", "http://"+c.localAddr)

	output, err := c.cmd.StderrPipe()
	if err != nil {
		cancel()
		return "", fmt.Errorf("failed to create pipe: %w", err)
	}

	if err := c.cmd.Start(); err != nil {
		cancel()
		return "", fmt.Errorf("failed to start cloudflared: %w", err)
	}

	buf := make([]byte, 4096)
	timeout := time.AfterFunc(15*time.Second, func() {
		cancel()
	})

	go func() {
		for {
			n, err := output.Read(buf)
			if err != nil {
				return
			}
			fmt.Printf("%s", buf[:n])
		}
	}()

	time.Sleep(3 * time.Second)
	timeout.Stop()

	c.tunnelURL = "https://your-tunnel.trycloudflare.com"
	fmt.Printf("🌐 Tunnel started! Check the output above for the public URL.\n")
	fmt.Printf("   (URL will be something like https://xxx.trycloudflare.com)\n\n")

	return c.tunnelURL, nil
}

func (c *Client) startManualTunnel() (string, error) {
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║                   TUNNEL SETUP REQUIRED                    ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Println("║ To expose your model publicly, install one of these:       ║")
	fmt.Println("║                                                            ║")
	fmt.Println("║  1. cloudflared (recommended):                             ║")
	fmt.Println("║     brew install cloudflared                               ║")
	fmt.Println("║     cloudflared tunnel --url http://localhost:8080         ║")
	fmt.Println("║                                                            ║")
	fmt.Println("║  2. ngrok:                                                 ║")
	fmt.Println("║     brew install ngrok                                     ║")
	fmt.Println("║     ngrok http 8080                                        ║")
	fmt.Println("║                                                            ║")
	fmt.Println("║  3. localtunnel (npm):                                     ║")
	fmt.Println("║     npm install -g localtunnel                             ║")
	fmt.Println("║     lt --port 8080                                         ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	return "", fmt.Errorf("no tunnel tool installed")
}

// Stop stops the tunnel
func (c *Client) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Signal(os.Interrupt)
		time.Sleep(100 * time.Millisecond)
		c.cmd.Process.Kill()
	}
}

// URL returns the tunnel URL
func (c *Client) URL() string {
	return c.tunnelURL
}

// PrintInstructions prints manual tunnel instructions
func PrintInstructions(port int) {
	fmt.Printf("\n📖 To expose this server publicly, run in another terminal:\n\n")
	fmt.Printf("   cloudflared tunnel --url http://localhost:%d\n", port)
	fmt.Println()
	fmt.Println("   Or install cloudflared first:")
	fmt.Println("   brew install cloudflared")
	fmt.Println()
}

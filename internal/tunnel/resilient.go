package tunnel

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// ResilientTunnel wraps a tunnel client with auto-reconnect capabilities
type ResilientTunnel struct {
	client       *SimpleTunnelClient
	maxRetries   int
	baseBackoff  time.Duration
	maxBackoff   time.Duration
	onConnect    func(url string)
	onDisconnect func()
	onReconnect  func(url string, attempt int)

	mu        sync.RWMutex
	connected bool
	url       string
	stopCh    chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
}

// ResilientTunnelConfig configures the resilient tunnel
type ResilientTunnelConfig struct {
	MaxRetries   int
	BaseBackoff  time.Duration
	MaxBackoff   time.Duration
	OnConnect    func(url string)
	OnDisconnect func()
	OnReconnect  func(url string, attempt int)
}

// DefaultResilientConfig returns sensible defaults
func DefaultResilientConfig() ResilientTunnelConfig {
	return ResilientTunnelConfig{
		MaxRetries:   10,
		BaseBackoff:  5 * time.Second,
		MaxBackoff:   5 * time.Minute,
		OnConnect:    func(url string) {},
		OnDisconnect: func() {},
		OnReconnect:  func(url string, attempt int) {},
	}
}

// NewResilientTunnel creates a new resilient tunnel client
func NewResilientTunnel(client *SimpleTunnelClient, config ResilientTunnelConfig) *ResilientTunnel {
	ctx, cancel := context.WithCancel(context.Background())

	return &ResilientTunnel{
		client:       client,
		maxRetries:   config.MaxRetries,
		baseBackoff:  config.BaseBackoff,
		maxBackoff:   config.MaxBackoff,
		onConnect:    config.OnConnect,
		onDisconnect: config.OnDisconnect,
		onReconnect:  config.OnReconnect,
		stopCh:       make(chan struct{}),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Start begins the tunnel connection with auto-reconnect support
func (rt *ResilientTunnel) Start() (string, error) {
	url, err := rt.connectWithRetry()
	if err != nil {
		return "", err
	}

	// Start monitoring connection
	go rt.monitorConnection()

	return url, nil
}

// connectWithRetry attempts to connect with exponential backoff
func (rt *ResilientTunnel) connectWithRetry() (string, error) {
	for attempt := 1; attempt <= rt.maxRetries; attempt++ {
		url, err := rt.client.Start()
		if err == nil {
			rt.mu.Lock()
			rt.connected = true
			rt.url = url
			rt.mu.Unlock()

			if attempt == 1 {
				rt.onConnect(url)
				log.Printf("✅ Tunnel connected: %s", url)
			} else {
				rt.onReconnect(url, attempt)
				log.Printf("✅ Tunnel reconnected after %d attempts: %s", attempt, url)
			}

			return url, nil
		}

		if attempt < rt.maxRetries {
			backoff := rt.calculateBackoff(attempt)

			if attempt == 1 {
				log.Printf("🔴 Tunnel failed: %v", err)
			} else {
				log.Printf("🔴 Tunnel failed (attempt %d/%d): %v", attempt, rt.maxRetries, err)
			}

			log.Printf("   Retrying in %v...", backoff)

			select {
			case <-time.After(backoff):
				continue
			case <-rt.stopCh:
				return "", fmt.Errorf("tunnel stopped during retry")
			case <-rt.ctx.Done():
				return "", fmt.Errorf("tunnel context cancelled")
			}
		}
	}

	return "", fmt.Errorf("tunnel failed after %d attempts", rt.maxRetries)
}

// calculateBackoff implements exponential backoff with jitter
func (rt *ResilientTunnel) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: base * 2^attempt
	backoff := rt.baseBackoff * time.Duration(1<<uint(attempt-1))

	// Cap at max backoff
	if backoff > rt.maxBackoff {
		backoff = rt.maxBackoff
	}

	// Add jitter (±25%) to prevent thundering herd
	jitter := time.Duration(float64(backoff) * 0.25 * (2*randFloat() - 1))
	backoff = backoff + jitter

	return backoff
}

func randFloat() float64 {
	// Simple pseudo-random for jitter
	return float64(time.Now().UnixNano()%1000) / 1000.0
}

// monitorConnection watches the tunnel and reconnects if it drops
func (rt *ResilientTunnel) monitorConnection() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !rt.client.IsConnected() && rt.IsConnected() {
				log.Println("⚠️  Tunnel connection lost! Attempting to reconnect...")
				rt.mu.Lock()
				rt.connected = false
				rt.mu.Unlock()
				rt.onDisconnect()

				// Attempt reconnection
				_, err := rt.connectWithRetry()
				if err != nil {
					log.Printf("💥 Failed to reconnect tunnel: %v", err)
					// Continue monitoring, will retry on next tick
				}
			}

		case <-rt.stopCh:
			return
		case <-rt.ctx.Done():
			return
		}
	}
}

// IsConnected returns the current connection status
func (rt *ResilientTunnel) IsConnected() bool {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.connected
}

// URL returns the current tunnel URL
func (rt *ResilientTunnel) URL() string {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.url
}

// Stop gracefully shuts down the tunnel
func (rt *ResilientTunnel) Stop() {
	close(rt.stopCh)
	rt.cancel()
	rt.client.Stop()

	rt.mu.Lock()
	rt.connected = false
	rt.mu.Unlock()

	log.Println("🛑 Tunnel stopped")
}

// HandleSignals sets up graceful shutdown on SIGINT/SIGTERM
func (rt *ResilientTunnel) HandleSignals() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("\n📡 Received signal: %v", sig)
		log.Println("🛑 Shutting down gracefully...")
		rt.Stop()
		os.Exit(0)
	}()
}

// GetStatus returns detailed tunnel status
func (rt *ResilientTunnel) GetStatus() map[string]interface{} {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	return map[string]interface{}{
		"connected":        rt.connected,
		"url":              rt.url,
		"client_connected": rt.client.IsConnected(),
	}
}

package config

import (
	"fmt"
	"log"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches config file for changes
type Watcher struct {
	configPath string
	config     *Config
	onChange   func(*Config)
	watcher    *fsnotify.Watcher
	stop       chan bool
	mu         sync.RWMutex
}

// NewWatcher creates a new config watcher
func NewWatcher(configPath string, onChange func(*Config)) (*Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	cw := &Watcher{
		configPath: configPath,
		onChange:   onChange,
		watcher:    watcher,
		stop:       make(chan bool),
	}

	return cw, nil
}

// Start starts watching the config file
func (cw *Watcher) Start() error {
	// Watch the directory containing the config file
	// This is necessary because some editors replace the file instead of modifying it
	dir := filepath.Dir(cw.configPath)
	if err := cw.watcher.Add(dir); err != nil {
		return fmt.Errorf("failed to watch config directory: %w", err)
	}

	go cw.watch()
	log.Println("🔄 Config hot-reload enabled - watching for changes...")
	return nil
}

// Stop stops the watcher
func (cw *Watcher) Stop() {
	close(cw.stop)
	cw.watcher.Close()
}

// watch is the main watch loop
func (cw *Watcher) watch() {
	for {
		select {
		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}
			// Check if the event is for our config file
			if filepath.Base(event.Name) == filepath.Base(cw.configPath) {
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
					cw.reload()
				}
			}
		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("⚠️ Config watcher error: %v", err)
		case <-cw.stop:
			return
		}
	}
}

// reload reloads the config file
func (cw *Watcher) reload() {
	log.Println("🔄 Config file changed, reloading...")

	newConfig, err := Load(cw.configPath)
	if err != nil {
		log.Printf("❌ Failed to reload config: %v", err)
		return
	}

	cw.mu.Lock()
	cw.config = newConfig
	cw.mu.Unlock()

	// Notify callback
	if cw.onChange != nil {
		cw.onChange(newConfig)
	}

	log.Println("✅ Config reloaded successfully")
}

// GetConfig returns the current config
func (cw *Watcher) GetConfig() *Config {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.config
}

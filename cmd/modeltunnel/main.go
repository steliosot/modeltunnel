package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modeltunnel/modeltunnel/internal/config"
	"github.com/modeltunnel/modeltunnel/internal/db"
	"github.com/modeltunnel/modeltunnel/internal/detect"
	"github.com/modeltunnel/modeltunnel/internal/keys"
	"github.com/modeltunnel/modeltunnel/internal/providers"
	"github.com/modeltunnel/modeltunnel/internal/server"
	"github.com/modeltunnel/modeltunnel/internal/tunnel"
	"github.com/modeltunnel/modeltunnel/internal/upstream"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "modeltunnel",
		Short: "Expose local models with OpenAI-compatible API",
		Long:  `Modeltunnel - ngrok for models. Expose your local Ollama models with auth + keys.`,
	}
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/modeltunnel/config.yaml)")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(keyCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := cfgFile
		if cfgPath == "" {
			cfgPath = config.GetConfigPath()
		}

		if _, err := os.Stat(cfgPath); err == nil {
			return fmt.Errorf("config file already exists at %s", cfgPath)
		}

		cfg := config.DefaultConfig()

		if err := cfg.Save(cfgPath); err != nil {
			return fmt.Errorf("failed to create config file: %w", err)
		}

		fmt.Printf("✅ Created config file at %s\n", cfgPath)
		fmt.Println("\nNext steps:")
		fmt.Println("  1. Add an upstream: modeltunnel up --ollama --model llama3")
		fmt.Println("  2. Create an API key: modeltunnel key create alice")

		return nil
	},
}

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Start the modeltunnel server",
	Long:  `Start the server and optionally create a public tunnel.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ollama, _ := cmd.Flags().GetBool("ollama")
		model, _ := cmd.Flags().GetString("model")
		tunnelFlag, _ := cmd.Flags().GetBool("tunnel")
		port, _ := cmd.Flags().GetInt("port")

		cfgPath := cfgFile
		if cfgPath == "" {
			cfgPath = config.GetConfigPath()
		}

		var cfg *config.Config
		var err error

		if _, err = os.Stat(cfgPath); err == nil {
			cfg, err = config.Load(cfgPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
		} else {
			cfg = config.DefaultConfig()
		}

		if port != 0 {
			cfg.Server.Port = port
		}

		// Get flags
		bindAll, _ := cmd.Flags().GetBool("bind-all")
		host, _ := cmd.Flags().GetString("host")

		// Auto-detect environment and bind to 0.0.0.0 if needed
		if host != "" {
			cfg.Server.Host = host
		} else if bindAll {
			cfg.Server.Host = "0.0.0.0"
			fmt.Println("Binding to 0.0.0.0 (--bind-all flag specified)")
		} else if cfg.Server.Host == "127.0.0.1" && detect.ShouldBindPublic() {
			fmt.Println(detect.GetBindingMessage())
			cfg.Server.Host = "0.0.0.0"
		} else {
			_, description := detect.GetEnvironment()
			fmt.Printf("%s\n", description)
		}

		manager := upstream.NewManager()

		if ollama {
			if model == "" {
				model = "llama3"
			}
			ollamaBaseURL := "http://127.0.0.1:11434"
			if def, ok := cfg.Upstreams["default"]; ok && def.Type == "ollama" && def.BaseURL != "" {
				ollamaBaseURL = def.BaseURL
			}
			ollamaUpstream := upstream.NewOllamaUpstream(ollamaBaseURL, model)
			manager.Register("default", ollamaUpstream)

			cfg.Upstreams["default"] = config.Upstream{
				Type:    "ollama",
				BaseURL: ollamaBaseURL,
				Model:   model,
			}
		} else {
			for name, u := range cfg.Upstreams {
				switch u.Type {
				case "ollama":
					ollamaUpstream := upstream.NewOllamaUpstream(u.BaseURL, u.Model)
					manager.Register(name, ollamaUpstream)
				}
			}
		}

		if len(manager.List()) == 0 {
			return fmt.Errorf("no upstreams configured. Use --ollama flag or add upstreams to config")
		}

		// Initialize database for key persistence
		dbPath := db.GetDBPath()
		database, err := db.New(dbPath)
		if err != nil {
			fmt.Printf("⚠️  Failed to initialize database: %v\n", err)
			fmt.Println("   Falling back to in-memory key storage (keys will not persist)")
		}

		var keystore *keys.Store
		if database != nil {
			keystore = keys.NewStoreWithDB(database)
			fmt.Printf("💾 Key database: %s\n", dbPath)
		} else {
			keystore = keys.NewStore()
		}

		// Migrate keys from config to database (if any)
		if len(cfg.Keys) > 0 && database != nil {
			keystoreKeys := make([]keys.KeyConfig, len(cfg.Keys))
			for i, k := range cfg.Keys {
				keystoreKeys[i] = keys.KeyConfig{
					Name:             k.Name,
					Key:              k.Key,
					AllowedUpstreams: k.AllowedUpstreams,
					Policy:           k.Policy,
				}
			}
			keystore.ReloadFromConfig(keystoreKeys)
			fmt.Printf("📥 Migrated %d keys from config to database\n", len(cfg.Keys))
			// Note: We keep keys in config as backup - don't delete them!
			// The database is the source of truth when it exists
		}

		// Create admin key if none exists
		if len(keystore.List()) == 0 {
			adminKey := keystore.Create("admin", []string{}, "default")
			fmt.Printf("🔑 Admin key created: %s\n", adminKey.Key)
		}

		// Initialize provider store for external API providers
		var providerStore *providers.ProviderStore
		if database != nil {
			providerStore, err = providers.NewProviderStore(database.Conn())
			if err != nil {
				fmt.Printf("⚠️  Failed to initialize provider store: %v\n", err)
				fmt.Println("   External provider features will not be available")
			} else {
				// Load providers from config
				for _, providerCfg := range cfg.Providers {
					// Check if provider already exists with same name and type
					allProviders, _ := providerStore.List()
					exists := false
					for _, p := range allProviders {
						if p.Name == providerCfg.Name && p.Type == providerCfg.Type {
							exists = true
							break
						}
					}

					if exists {
						fmt.Printf("ℹ️  Provider '%s' already exists, skipping\n", providerCfg.Name)
						continue
					}

					// Generate ID from type and timestamp
					id := fmt.Sprintf("%s-legacy-%d", providerCfg.Type, time.Now().UnixNano())

					provider := &providers.ProviderConfig{
						ID:         id,
						Name:       providerCfg.Name,
						Type:       providerCfg.Type,
						APIKey:     providerCfg.APIKey,
						BaseURL:    providerCfg.BaseURL,
						Models:     providerCfg.Models,
						RateLimit:  providerCfg.RateLimit,
						Priority:   providerCfg.Priority,
						IsActive:   true,
						TrackCosts: true,
					}

					if err := providerStore.Create(provider); err != nil {
						fmt.Printf("⚠️  Failed to migrate provider '%s' to database: %v\n", providerCfg.Name, err)
					} else {
						fmt.Printf("📦 Imported provider '%s' from config\n", providerCfg.Name)
					}
				}
			}
		}

		srv := server.NewServer(cfg, manager, keystore, providerStore, cfgPath)

		var tunnelClient interface {
			Start() (string, error)
			Stop()
			SetStatusCallback(func(connected bool, url string))
		}
		var tunnelURL string
		if tunnelFlag {
			subdomain, _ := cmd.Flags().GetString("subdomain")

			tunnelClient = tunnel.NewLocalTunnelClient(srv.Addr(), subdomain)

			// Set up status callback
			tunnelClient.SetStatusCallback(func(connected bool, url string) {
				srv.SetTunnelStatus(connected, url)
			})

			publicURL, err := tunnelClient.Start()
			if err != nil {
				fmt.Printf("⚠️  Failed to start tunnel: %v\n", err)
				fmt.Println("   Server is still running locally.")
			} else {
				tunnelURL = publicURL
				// Immediately set tunnel status so dashboard shows it
				srv.SetTunnelStatus(true, publicURL)
				fmt.Printf("\n🌐 Public URL: %s/v1\n", publicURL)
				fmt.Printf("   URL saved to: ~/.config/modeltunnel/tunnel.url\n")
				fmt.Println("\n   Dashboard:")
				fmt.Printf("   - Status: http://%s/admin (shows tunnel health)\n", srv.Addr())
			}
		}

		// Print usage info
		fmt.Println("\n📖 Usage:")
		if tunnelURL != "" {
			fmt.Printf("   Public: https://%s/v1\n", tunnelURL)
			fmt.Printf("   Local:  http://%s/v1\n", srv.Addr())
		} else {
			fmt.Printf("   Local:  http://%s/v1\n", srv.Addr())
		}
		fmt.Println("\n   Dashboard:")
		fmt.Printf("   http://%s/admin\n", srv.Addr())
		fmt.Println("\n   OpenCode/Cursor settings:")
		if tunnelURL != "" {
			fmt.Printf("   - Base URL: https://%s/v1\n", tunnelURL)
		} else {
			fmt.Printf("   - Base URL: http://%s/v1\n", srv.Addr())
		}
		fmt.Printf("   - Key:      %s\n", keystore.List()[0].Key)
		fmt.Println("\n   Available models:")
		for _, name := range manager.List() {
			fmt.Printf("   - %s/*\n", name)
		}
		fmt.Println("\n   Press Ctrl+C to stop")
		fmt.Println()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigChan
			fmt.Println("\n🛑 Shutting down...")

			// Stop tunnel first
			if tunnelClient != nil {
				tunnelClient.Stop()
			}

			// Stop server
			if err := srv.Stop(ctx); err != nil {
				fmt.Printf("⚠️  Error stopping server: %v\n", err)
			}

			// Close database connection
			if database != nil {
				fmt.Println("💾 Closing database...")
				if err := database.Close(); err != nil {
					fmt.Printf("⚠️  Error closing database: %v\n", err)
				}
			}

			cancel()
		}()

		return srv.Start()
	},
}

var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Manage API keys",
}

var keyCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		models, _ := cmd.Flags().GetStringSlice("models")
		rate, _ := cmd.Flags().GetString("rate")

		cfgPath := cfgFile
		if cfgPath == "" {
			cfgPath = config.GetConfigPath()
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Initialize database for key persistence
		dbPath := db.GetDBPath()
		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer database.Close()

		keystore := keys.NewStoreWithDB(database)

		policy := "default"
		if rate != "" {
			policy = fmt.Sprintf("custom-%s", name)
			cfg.Policies[policy] = config.Policy{
				RateLimit: rate,
				MaxTokens: 4096,
			}
			cfg.Save(cfgPath)
		}

		key := keystore.Create(name, models, policy)

		fmt.Printf("✅ Created key for '%s'\n", name)
		fmt.Printf("   API Key: %s\n", key.Key)
		if len(models) > 0 {
			fmt.Printf("   Models:  %v\n", models)
		}
		if rate != "" {
			fmt.Printf("   Rate:    %s\n", rate)
		}

		return nil
	},
}

var keyListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all API keys",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Initialize database
		dbPath := db.GetDBPath()
		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer database.Close()

		keystore := keys.NewStoreWithDB(database)
		keyList := keystore.List()

		if len(keyList) == 0 {
			fmt.Println("No keys found")
			return nil
		}

		fmt.Println("API Keys:")
		fmt.Println("---------")
		for _, k := range keyList {
			masked := k.Key
			if len(masked) > 20 {
				masked = masked[:20] + "..."
			}
			fmt.Printf("  %-15s %s\n", k.Name, masked)
		}

		return nil
	},
}

var keyRevokeCmd = &cobra.Command{
	Use:   "revoke [name]",
	Short: "Revoke an API key",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfgPath := cfgFile
		if cfgPath == "" {
			cfgPath = config.GetConfigPath()
		}

		// Initialize database
		dbPath := db.GetDBPath()
		database, err := db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		defer database.Close()

		keystore := keys.NewStoreWithDB(database)

		if !keystore.Revoke(name) {
			return fmt.Errorf("key '%s' not found", name)
		}

		fmt.Printf("✅ Revoked key '%s'\n", name)
		return nil
	},
}

func init() {
	upCmd.Flags().Bool("ollama", false, "Use Ollama upstream")
	upCmd.Flags().String("model", "llama3", "Model name (for Ollama)")
	upCmd.Flags().Bool("tunnel", false, "Create a public tunnel using LocalTunnel")
	upCmd.Flags().String("subdomain", "", "Custom subdomain for tunnel (e.g., 'mymodel' -> mymodel.loca.lt)")
	upCmd.Flags().Int("port", 0, "Server port (overrides config)")
	upCmd.Flags().Bool("bind-all", false, "Bind to 0.0.0.0 (all interfaces) for external access")
	upCmd.Flags().String("host", "", "Bind to specific host (overrides config)")

	keyCmd.AddCommand(keyCreateCmd)
	keyCmd.AddCommand(keyListCmd)
	keyCmd.AddCommand(keyRevokeCmd)

	keyCreateCmd.Flags().StringSlice("models", []string{}, "Allowed models")
	keyCreateCmd.Flags().String("rate", "", "Rate limit (e.g., 60/min)")
}

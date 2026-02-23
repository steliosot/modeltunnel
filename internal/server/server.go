package server

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/modeltunnel/modeltunnel/internal/config"
	"github.com/modeltunnel/modeltunnel/internal/gateway"
	"github.com/modeltunnel/modeltunnel/internal/jobs"
	"github.com/modeltunnel/modeltunnel/internal/keys"
	"github.com/modeltunnel/modeltunnel/internal/models"
	"github.com/modeltunnel/modeltunnel/internal/network"
	"github.com/modeltunnel/modeltunnel/internal/providers"
	"github.com/modeltunnel/modeltunnel/internal/router"
	"github.com/modeltunnel/modeltunnel/internal/tunnel"
	"github.com/modeltunnel/modeltunnel/internal/upstream"
	"github.com/modeltunnel/modeltunnel/pkg/openai"
	"gopkg.in/yaml.v3"
)

//go:embed static/dashboard.html
var dashboardHTML string

//go:embed static/landing.html
var landingHTML string

// LogEntry represents a request log entry
type LogEntry struct {
	Time     string `json:"time"`
	Method   string `json:"method"`
	Path     string `json:"path"`
	Status   int    `json:"status"`
	Duration string `json:"duration"`
}

// LogHub broadcasts logs to connected WebSocket clients
type LogHub struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan LogEntry
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.RWMutex
}

func newLogHub() *LogHub {
	return &LogHub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan LogEntry, 100),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

func (h *LogHub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mu.Unlock()
		case entry := <-h.broadcast:
			h.mu.RLock()
			clientCount := len(h.clients)
			if clientCount > 0 {
				data, _ := json.Marshal(entry)
				for client := range h.clients {
					if err := client.WriteMessage(websocket.TextMessage, data); err != nil {
						client.Close()
						delete(h.clients, client)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *LogHub) broadcastEntry(entry LogEntry) {
	select {
	case h.broadcast <- entry:
	default:
		// Channel full, drop log
	}
}

// ResponseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher for streaming support
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack implements http.Hijacker for WebSocket support
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("response writer does not support hijacking")
}

// adminAuth wraps admin routes with optional HTTP Basic Auth
// adminAuth wraps admin routes with optional HTTP Basic Auth
func (s *Server) adminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if admin auth is enabled
		adminConfig := s.config.Server.Admin
		if !adminConfig.Enabled || adminConfig.Username == "" || adminConfig.Password == "" {
			// No auth configured - allow public admin access
			next(w, r)
			return
		}

		// Basic Auth check
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Modeltunnel Admin"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Parse Basic Auth
		expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(adminConfig.Username+":"+adminConfig.Password))
		if auth != expectedAuth {
			w.Header().Set("WWW-Authenticate", `Basic realm="Modeltunnel Admin"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for dashboard
	},
}

// Server represents the HTTP server
type Server struct {
	mux           *http.ServeMux
	server        *http.Server
	upstreams     *upstream.Manager
	keystore      *keys.Store
	providerStore *providers.ProviderStore
	config        *config.Config
	rateLimiter   *gateway.RateLimiter
	intentRouter  *router.IntentRouter
	jobStore      *jobs.Store
	jobQueue      *jobs.Queue
	jobWorkers    *jobs.WorkerPool
	logHub        *LogHub
	configWatcher *config.Watcher
	configPath    string
	mu            sync.RWMutex
	tunnelStatus  *TunnelStatus
	tunnelClient  tunnelClient
	modelManager  *models.Manager
}

type tunnelClient interface {
	Start() (string, error)
	Stop()
	SetStatusCallback(func(connected bool, url string))
}

// TunnelStatus holds the current tunnel connection status
type TunnelStatus struct {
	Connected bool   `json:"connected"`
	URL       string `json:"url,omitempty"`
	Message   string `json:"message,omitempty"`
}

// NewServer creates a new server
func NewServer(cfg *config.Config, upstreams *upstream.Manager, keystore *keys.Store, providerStore *providers.ProviderStore, configPath string) *Server {
	// Initialize job system
	jobStore := jobs.NewStore()
	jobQueue := jobs.NewQueue(1000)
	jobWorkers := jobs.NewWorkerPool(3, jobQueue, jobStore, upstreams)
	jobWorkers.Start()

	// Get Ollama base URL from config for model manager
	ollamaBaseURL := ""
	if def, ok := cfg.Upstreams["default"]; ok && def.Type == "ollama" && def.BaseURL != "" {
		ollamaBaseURL = def.BaseURL
	}

	s := &Server{
		mux:           http.NewServeMux(),
		server:        nil,
		upstreams:     upstreams,
		keystore:      keystore,
		providerStore: providerStore,
		config:        cfg,
		intentRouter:  router.NewIntentRouterFromConfig(cfg.Intents),
		jobStore:      jobStore,
		jobQueue:      jobQueue,
		jobWorkers:    jobWorkers,
		logHub:        newLogHub(),
		configPath:    configPath,
		modelManager:  models.NewManager(ollamaBaseURL),
	}

	// Setup routes
	s.mux.HandleFunc("/", s.handleLanding)
	s.mux.HandleFunc("/v1/models", s.handleModels)
	s.mux.HandleFunc("/v1/chat/completions", s.handleChatCompletions)
	s.mux.HandleFunc("/v1/async", s.handleAsyncSubmit)
	s.mux.HandleFunc("/v1/jobs/", s.handleJobStatus)
	s.mux.HandleFunc("/health", s.handleHealth)

	// Dashboard routes with optional authentication
	s.mux.HandleFunc("/admin", s.adminAuth(s.handleDashboard))
	s.mux.HandleFunc("/admin/api/keys", s.adminAuth(s.handleAdminKeys))
	s.mux.HandleFunc("/admin/api/keys/", s.adminAuth(s.handleAdminKeyDetail))
	s.mux.HandleFunc("/admin/api/logs", s.adminAuth(s.handleAdminLogs))
	s.mux.HandleFunc("/admin/api/tunnel", s.adminAuth(s.handleAdminTunnel))
	s.mux.HandleFunc("/admin/api/config", s.adminAuth(s.handleAdminConfig))
	s.mux.HandleFunc("/admin/api/models/pull", s.adminAuth(s.handleAdminModelsPull))
	s.mux.HandleFunc("/admin/api/models/pull/", s.adminAuth(s.handleAdminModelsPullProgress))
	s.mux.HandleFunc("/admin/api/models/", s.adminAuth(s.handleAdminModelsDelete))
	s.mux.HandleFunc("/admin/api/models", s.adminAuth(s.handleAdminModels))

	// Provider routes
	s.mux.HandleFunc("/admin/api/providers", s.adminAuth(s.handleAdminProviders))
	s.mux.HandleFunc("/admin/api/providers/", s.adminAuth(s.handleAdminProviderDetail))
	s.mux.HandleFunc("/admin/api/providers/types", s.adminAuth(s.handleAdminProviderTypes))
	s.mux.HandleFunc("/admin/api/providers/test", s.adminAuth(s.handleAdminProviderTest))

	// Config generation route
	s.mux.HandleFunc("/admin/api/config/generate", s.adminAuth(s.handleGenerateConfig))

	// Network endpoints
	s.mux.HandleFunc("/admin/api/network", s.adminAuth(s.handleNetwork))

	// Setup rate limiter
	if policy, ok := cfg.Policies["default"]; ok {
		if limit, window, err := gateway.ParseRateLimit(policy.RateLimit); err == nil {
			s.rateLimiter = gateway.NewRateLimiter(limit, window)
			s.rateLimiter.StartCleanup(5 * time.Minute)
		}
	}

	// Wrap with middleware
	var handler http.Handler = s.mux
	handler = s.authMiddleware(handler)
	if s.rateLimiter != nil {
		handler = s.rateLimiter.Middleware(handler)
	}
	handler = s.loggingMiddleware(handler)

	s.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: handler,
	}

	// Start log hub
	go s.logHub.run()

	// Start config watcher for hot-reload
	if configPath != "" {
		s.startConfigWatcher(configPath)
	}

	return s
}

// startConfigWatcher starts watching config file for changes
func (s *Server) startConfigWatcher(configPath string) {
	watcher, err := config.NewWatcher(configPath, func(newCfg *config.Config) {
		s.reloadConfig(newCfg)
	})
	if err != nil {
		fmt.Printf("⚠️ Failed to start config watcher: %v\n", err)
		return
	}

	s.configWatcher = watcher
	if err := watcher.Start(); err != nil {
		fmt.Printf("⚠️ Failed to start config watcher: %v\n", err)
	}
}

// reloadConfig reloads configuration without restart
func (s *Server) reloadConfig(newCfg *config.Config) {
	fmt.Println("🔄 Reloading configuration...")

	s.mu.Lock()
	defer s.mu.Unlock()

	// Update config reference
	s.config = newCfg

	// Convert config keys to keystore keys
	keystoreKeys := make([]keys.KeyConfig, len(newCfg.Keys))
	for i, k := range newCfg.Keys {
		keystoreKeys[i] = keys.KeyConfig{
			Name:             k.Name,
			Key:              k.Key,
			AllowedUpstreams: k.AllowedUpstreams,
			Policy:           k.Policy,
		}
	}

	// Reload keys (preserve usage stats)
	s.keystore.ReloadFromConfig(keystoreKeys)

	// Reload rate limiter if policy changed
	if policy, ok := newCfg.Policies["default"]; ok {
		if limit, window, err := gateway.ParseRateLimit(policy.RateLimit); err == nil {
			if s.rateLimiter != nil {
				// Update existing rate limiter
				s.rateLimiter.UpdateLimit(limit, window)
			} else {
				// Create new rate limiter
				s.rateLimiter = gateway.NewRateLimiter(limit, window)
				s.rateLimiter.StartCleanup(5 * time.Minute)
			}
		}
	}

	// Reload upstreams (for dynamic upstream changes)
	for name, u := range newCfg.Upstreams {
		switch u.Type {
		case "ollama":
			// Check if upstream already exists
			if _, ok := s.upstreams.Get(name); !ok {
				// Register new upstream
				ollamaUpstream := upstream.NewOllamaUpstream(u.BaseURL, u.Model)
				s.upstreams.Register(name, ollamaUpstream)
				fmt.Printf("✅ Added upstream: %s\n", name)
			}
		}
	}

	fmt.Println("✅ Configuration reloaded successfully")
}

// Start starts the server
func (s *Server) Start() error {
	fmt.Printf("🚀 Server starting on http://%s\n", s.server.Addr)
	fmt.Printf("📊 Dashboard available at http://%s/admin\n", s.server.Addr)
	return s.server.ListenAndServe()
}

// Stop stops the server
func (s *Server) Stop(ctx context.Context) error {
	if s.configWatcher != nil {
		s.configWatcher.Stop()
	}
	return s.server.Shutdown(ctx)
}

// getEffectivePolicy returns the effective policy for a given policy name and model
func (s *Server) getEffectivePolicy(policyName, modelName string) config.Policy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get base policy
	policy, ok := s.config.Policies[policyName]
	if !ok {
		policy = s.config.Policies["default"]
	}

	// Get model-specific overrides
	return policy.GetEffectivePolicy(modelName)
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Get Ollama models from upstreams
	models, err := s.upstreams.AllModels(ctx)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Add external provider models
	if s.providerStore != nil {
		providerList, err := s.providerStore.ListActive()
		if err == nil {
			for _, provider := range providerList {
				for _, modelID := range provider.Models {
					models = append(models, openai.Model{
						ID:      fmt.Sprintf("%s/%s", provider.ID, modelID),
						Object:  "model",
						Created: time.Now().Unix(),
						OwnedBy: provider.Type,
					})
				}
			}
		}
	}

	s.writeJSON(w, http.StatusOK, openai.ModelList{
		Object: "list",
		Data:   models,
	})
}

func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req openai.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Check for intent-based routing
	intent := r.Header.Get("X-Model-Intent")
	if intent != "" || req.Model == "auto" {
		// Get available models from upstreams and providers
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		availableModels, _ := s.upstreams.AllModels(ctx)

		// Add external provider models to available models
		if s.providerStore != nil {
			providerList, err := s.providerStore.ListActive()
			if err == nil {
				for _, provider := range providerList {
					for _, modelID := range provider.Models {
						availableModels = append(availableModels, openai.Model{
							ID:      fmt.Sprintf("%s/%s", provider.ID, modelID),
							Object:  "model",
							Created: time.Now().Unix(),
							OwnedBy: provider.Type,
						})
					}
				}
			}
		}

		// Convert to string slice
		modelIDs := make([]string, len(availableModels))
		for i, m := range availableModels {
			modelIDs[i] = m.ID
		}

		// Route to best model for intent
		routedModel, temp, maxTok := s.intentRouter.Route(intent, modelIDs)
		req.Model = routedModel

		// Apply intent-specific parameters if not already set
		if req.Temperature == nil {
			req.Temperature = &temp
		}
		if req.MaxTokens == nil {
			req.MaxTokens = &maxTok
		}

		// Add routing info to response headers
		w.Header().Set("X-Routed-Model", routedModel)
		if intent != "" {
			w.Header().Set("X-Model-Intent", intent)
		}
	}

	// Get upstream from model name
	upstreamName := s.config.Server.Host
	modelName := req.Model
	if strings.Contains(req.Model, "/") {
		parts := strings.SplitN(req.Model, "/", 2)
		upstreamName = parts[0]
		modelName = parts[1]
		req.Model = modelName
	}

	// Check if this is an external provider
	if s.providerStore != nil {
		provider, err := s.providerStore.Get(upstreamName)
		if err == nil && provider.IsActive {
			// Route to external provider
			s.handleProviderChatCompletion(w, r, provider, &req)
			return
		}
	}

	up, ok := s.upstreams.Get(upstreamName)
	if !ok {
		up, ok = s.upstreams.GetDefault()
		if !ok {
			s.writeError(w, http.StatusBadRequest, "unknown upstream")
			return
		}
	}

	// Get API key from context (set by authMiddleware)
	key, ok := r.Context().Value("key").(*keys.Key)
	if !ok {
		s.writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Get effective policy for this model
	policy := s.getEffectivePolicy(key.Policy, modelName)

	// Set per-model rate limit for this key+model combination
	if s.rateLimiter != nil && policy.RateLimit != "" {
		if limit, window, err := gateway.ParseRateLimit(policy.RateLimit); err == nil {
			s.rateLimiter.SetKeyLimit(key.Key+":"+modelName, limit, window)
		}
	}

	// Check per-model rate limit
	if s.rateLimiter != nil {
		info := s.rateLimiter.CheckRateLimitWithModel(key.Key, modelName)
		if !info.Allowed {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", fmt.Sprintf("%d", info.RetryAfter))
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", info.Limit))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", info.Remaining))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", info.ResetTime.Unix()))
			w.WriteHeader(http.StatusTooManyRequests)
			resetTimeStr := info.ResetTime.Format("2006-01-02T15:04:05Z")
			errorMsg := fmt.Sprintf(
				`{"error": {"code": "RATE_LIMIT_EXCEEDED", "message": "Rate limit exceeded for model '%s'", "retry_after": %d, "reset_time": "%s", "limit": %d, "student_friendly": "You've made too many requests to this model. Please wait %d seconds before trying again."}}`,
				modelName, info.RetryAfter, resetTimeStr, info.Limit, info.RetryAfter,
			)
			w.Write([]byte(errorMsg))
			return
		}
		// Set rate limit headers
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", info.Limit))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", info.Remaining))
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", info.ResetTime.Unix()))
	}

	// Apply model-specific max_tokens
	if policy.MaxTokens > 0 {
		if req.MaxTokens == nil || *req.MaxTokens > policy.MaxTokens {
			req.MaxTokens = &policy.MaxTokens
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	if req.Stream {
		s.handleStreamingResponse(w, r, ctx, up, &req)
	} else {
		s.handleStandardResponse(w, ctx, up, &req)
	}
}

func (s *Server) handleStandardResponse(w http.ResponseWriter, ctx context.Context, up upstream.Upstream, req *openai.ChatCompletionRequest) {
	resp, err := up.ChatCompletion(ctx, req)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStreamingResponse(w http.ResponseWriter, r *http.Request, ctx context.Context, up upstream.Upstream, req *openai.ChatCompletionRequest) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	err := up.ChatCompletionStream(ctx, req, func(chunk openai.ChatCompletionStreamResponse) {
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	})

	if err != nil {
		data, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// handleProviderChatCompletion routes requests to external providers (OpenAI, Anthropic)
func (s *Server) handleProviderChatCompletion(w http.ResponseWriter, r *http.Request, providerConfig *providers.ProviderConfig, req *openai.ChatCompletionRequest) {
	// Create provider instance
	provider, err := providers.ProviderFromConfig(providerConfig)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create provider: %v", err))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	// Track usage
	var inputTokens, outputTokens int64

	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		flusher, ok := w.(http.Flusher)
		if !ok {
			s.writeError(w, http.StatusInternalServerError, "streaming not supported")
			return
		}

		err := provider.ChatCompletionStream(ctx, req, func(chunk openai.ChatCompletionStreamResponse) {
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		})

		if err != nil {
			data, _ := json.Marshal(map[string]string{"error": err.Error()})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}

		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	} else {
		resp, err := provider.ChatCompletion(ctx, req)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		inputTokens = int64(resp.Usage.PromptTokens)
		outputTokens = int64(resp.Usage.CompletionTokens)
		s.writeJSON(w, http.StatusOK, resp)
	}

	// Update usage statistics
	if s.providerStore != nil && providerConfig.TrackCosts {
		cost := provider.EstimateCost(req.Model, inputTokens, outputTokens)
		_ = s.providerStore.UpdateUsage(providerConfig.ID, 1, inputTokens+outputTokens, cost)
		_ = s.providerStore.UpdateLastUsed(providerConfig.ID)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleLanding(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		s.writeError(w, http.StatusNotFound, "not found")
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(landingHTML))
}

// Dashboard handlers
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(dashboardHTML))
}

func (s *Server) handleAdminKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listKeys(w, r)
	case http.MethodPost:
		s.createKey(w, r)
	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleAdminKeyDetail(w http.ResponseWriter, r *http.Request) {
	// Extract key name from /admin/api/keys/{name}
	path := strings.TrimPrefix(r.URL.Path, "/admin/api/keys/")
	if path == "" || strings.Contains(path, "/") {
		s.writeError(w, http.StatusBadRequest, "invalid key name")
		return
	}

	if r.Method == http.MethodDelete {
		s.revokeKey(w, r, path)
	} else {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) listKeys(w http.ResponseWriter, r *http.Request) {
	keyList := s.keystore.List()

	totalRequests := int64(0)
	for _, k := range keyList {
		totalRequests += k.RequestCount
	}

	response := map[string]interface{}{
		"keys": keyList,
		"stats": map[string]interface{}{
			"active_keys":    len(keyList),
			"total_requests": totalRequests,
			"upstreams":      len(s.upstreams.List()),
		},
	}

	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) createKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name          string   `json:"name"`
		RateLimit     string   `json:"rate_limit"`
		AllowedModels []string `json:"allowed_models"`
		Upstreams     []string `json:"allowed_upstreams"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		s.writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	// Convert models to upstreams format
	allowedUpstreams := []string{}
	if len(req.Upstreams) > 0 {
		allowedUpstreams = req.Upstreams
	}
	if len(req.AllowedModels) > 0 {
		// For now, allow all upstreams if models specified
		// In the future, we could map models to specific upstreams
		allowedUpstreams = s.upstreams.List()
	}

	policy := "default"
	if req.RateLimit != "" && req.RateLimit != "60/min" {
		policy = fmt.Sprintf("custom-%s", req.Name)
		s.config.Policies[policy] = config.Policy{
			RateLimit: req.RateLimit,
			MaxTokens: 4096,
		}
		// Persist policy change
		_ = s.config.Save(s.configPath)
	}

	key := s.keystore.Create(req.Name, allowedUpstreams, policy)

	// Add to config for persistence
	s.config.Keys = append(s.config.Keys, config.KeyConfig{
		Name:             key.Name,
		Key:              key.Key,
		AllowedUpstreams: key.AllowedUpstreams,
		Policy:           key.Policy,
	})

	// Save config
	cfgPath := config.GetConfigPath()
	s.config.Save(cfgPath)

	s.writeJSON(w, http.StatusCreated, map[string]interface{}{
		"name": key.Name,
		"key":  key.Key,
	})
}

func (s *Server) revokeKey(w http.ResponseWriter, r *http.Request, name string) {
	if !s.keystore.Revoke(name) {
		s.writeError(w, http.StatusNotFound, "key not found")
		return
	}

	// Remove from config
	for i, k := range s.config.Keys {
		if k.Name == name {
			s.config.Keys = append(s.config.Keys[:i], s.config.Keys[i+1:]...)
			break
		}
	}

	// Save config
	cfgPath := config.GetConfigPath()
	s.config.Save(cfgPath)

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminLogs(w http.ResponseWriter, r *http.Request) {
	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("WebSocket upgrade failed: %v\n", err)
		return
	}

	fmt.Printf("WebSocket client connected from %s\n", r.RemoteAddr)
	s.logHub.register <- conn
}

func (s *Server) handleAdminTunnel(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.mu.RLock()
		status := s.tunnelStatus
		s.mu.RUnlock()

		if status == nil {
			// No tunnel started at all
			s.writeJSON(w, http.StatusOK, TunnelStatus{Connected: false})
			return
		}

		s.writeJSON(w, http.StatusOK, status)
		return

	case http.MethodPost:
		var req struct {
			Subdomain string `json:"subdomain"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		s.mu.Lock()
		if s.tunnelClient == nil {
			s.tunnelClient = tunnel.NewLocalTunnelClient(s.Addr(), req.Subdomain)
			s.tunnelClient.SetStatusCallback(func(connected bool, url string) {
				s.SetTunnelStatus(connected, url)
			})
		}
		existing := s.tunnelStatus
		s.mu.Unlock()

		if existing != nil && existing.Connected {
			s.writeJSON(w, http.StatusOK, existing)
			return
		}

		publicURL, err := s.tunnelClient.Start()
		if err != nil {
			s.SetTunnelStatus(false, "", err.Error())
			s.writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		s.SetTunnelStatus(true, publicURL)
		s.writeJSON(w, http.StatusOK, TunnelStatus{Connected: true, URL: publicURL})
		return

	case http.MethodDelete:
		s.mu.Lock()
		client := s.tunnelClient
		s.mu.Unlock()

		if client != nil {
			client.Stop()
		}
		s.SetTunnelStatus(false, "", "stopped")
		s.writeJSON(w, http.StatusOK, TunnelStatus{Connected: false, Message: "stopped"})
		return

	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
}

// SetTunnelStatus updates the tunnel connection status
func (s *Server) SetTunnelStatus(connected bool, url string, message ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	msg := ""
	if len(message) > 0 {
		msg = message[0]
	}
	s.tunnelStatus = &TunnelStatus{
		Connected: connected,
		URL:       url,
		Message:   msg,
	}
}

func (s *Server) SetTunnelClient(client tunnelClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tunnelClient = client
}

func (s *Server) handleAdminConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		data, err := os.ReadFile(s.configPath)
		configContent := string(data)

		// If config is missing or empty, use default config
		if err != nil || configContent == "" {
			defaultConfig := config.DefaultConfig()
			yamlData, _ := yaml.Marshal(defaultConfig)
			configContent = string(yamlData)
		}

		s.writeJSON(w, http.StatusOK, map[string]interface{}{
			"config":      configContent,
			"server_host": s.config.Server.Host,
			"server_port": s.config.Server.Port,
		})

	case http.MethodPost:
		var req struct {
			Config string `json:"config"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		// Validate YAML by trying to parse it
		var testConfig config.Config
		if err := yaml.Unmarshal([]byte(req.Config), &testConfig); err != nil {
			s.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid YAML: %v", err))
			return
		}

		// Write config to file
		if err := os.WriteFile(s.configPath, []byte(req.Config), 0600); err != nil {
			s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to write config: %v", err))
			return
		}

		w.WriteHeader(http.StatusOK)
		s.writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})

	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleGenerateConfig generates a new configuration based on available models and providers
func (s *Server) handleGenerateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	type modelInfo struct {
		id     string
		name   string
		size   int64
		source string
	}

	var allModels []modelInfo

	// Get models from local upstreams (Ollama, VLLM)
	if s.upstreams != nil {
		models, err := s.upstreams.AllModels(ctx)
		if err == nil {
			for _, m := range models {
				allModels = append(allModels, modelInfo{
					id:     m.ID,
					name:   m.ID,
					size:   m.Size,
					source: "local",
				})
			}
		}
	}

	// Get models from external providers if available
	if s.providerStore != nil {
		providerConfigs, err := s.providerStore.List()
		if err == nil {
			for _, p := range providerConfigs {
				if !p.IsActive {
					continue
				}
				provider, err := providers.ProviderFromConfig(p)
				if err != nil {
					continue
				}
				providerModels, err := provider.ListModels(ctx)
				if err != nil {
					continue
				}
				for _, m := range providerModels {
					if m.ID == "" {
						continue
					}
					sizes, _ := m.Size, 0
					allModels = append(allModels, modelInfo{
						id:     p.ID + "/" + m.ID,
						name:   m.ID,
						size:   sizes,
						source: "provider",
					})
				}
			}
		}
	}

	// Sort all models by size (largest first)
	sort.Slice(allModels, func(i, j int) bool {
		return allModels[i].size > allModels[j].size
	})

	// Separate local and provider models for priority ordering
	var localModels []modelInfo
	var providerModels []modelInfo
	for _, m := range allModels {
		if m.source == "local" {
			localModels = append(localModels, m)
		} else {
			providerModels = append(providerModels, m)
		}
	}

	// Generate intent priorities - take top 3 largest overall
	// Local models first, then providers, but keep size ordering within groups
	topModels := make([]string, 0)
	for i := 0; i < len(localModels) && i < 3; i++ {
		topModels = append(topModels, localModels[i].id)
	}
	for i := 0; i < len(providerModels) && len(topModels) < 3; i++ {
		topModels = append(topModels, providerModels[i].id)
	}

	// Code intent - top 3, prioritize different ones if possible
	topCodeModels := make([]string, 0)
	for i := 0; i < len(localModels) && len(topCodeModels) < 3; i++ {
		// Skip if already in chat list (distribute models)
		if i > 0 && i < len(topModels) && i < len(localModels)-1 {
			topCodeModels = append(topCodeModels, localModels[i].id)
		} else {
			topCodeModels = append(topCodeModels, localModels[i].id)
		}
	}
	for i := 0; i < len(providerModels) && len(topCodeModels) < 3; i++ {
		topCodeModels = append(topCodeModels, providerModels[i].id)
	}

	// Load existing config to get server settings
	currentCfg, err := config.Load(s.configPath)
	if err != nil {
		currentCfg = config.DefaultConfig()
	}

	// Generate new config with updated intents
	generated := struct {
		Server    config.ServerConfig        `yaml:"server"`
		Upstreams map[string]config.Upstream `yaml:"upstreams,omitempty"`
		Policies  map[string]config.Policy   `yaml:"policies,omitempty"`
		Intents   map[string]config.Intent   `yaml:"intents"`
		Keys      []config.KeyConfig         `yaml:"keys"`
		Providers []config.ProviderConfig    `yaml:"providers,omitempty"`
	}{
		Server:    currentCfg.Server,
		Upstreams: currentCfg.Upstreams,
		Policies:  currentCfg.Policies,
		Intents: map[string]config.Intent{
			"chat": {
				Priority:    topModels,
				Description: "General conversation, Q&A, support",
				Temperature: 0.7,
				MaxTokens:   2048,
			},
			"code": {
				Priority:    topCodeModels,
				Description: "Programming, debugging, technical assistance",
				Temperature: 0.2,
				MaxTokens:   4096,
			},
		},
		Keys:      currentCfg.Keys,
		Providers: currentCfg.Providers,
	}

	yamlData, err := yaml.Marshal(generated)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to generate config: %v", err))
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"config":      string(yamlData),
		"intents":     generated.Intents,
		"model_count": len(allModels),
	})
}

func (s *Server) handleAdminModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	models, err := s.upstreams.AllModels(ctx)
	if err != nil {
		s.writeJSON(w, http.StatusOK, openai.ModelList{
			Object: "list",
			Data:   []openai.Model{},
		})
		return
	}

	// Enhance models with config status
	enhancedModels := make([]openai.Model, len(models))
	modelNameToOriginal := make(map[string]string) // Map to track original model names without upstream prefix

	for i, model := range models {
		enhancedModels[i] = model

		// Store mapping for intent checking (e.g., "default/mistral" -> "mistral")
		if slashIndex := strings.Index(model.ID, "/"); slashIndex >= 0 {
			modelNameToOriginal[model.ID[slashIndex+1:]] = model.ID
		} else {
			modelNameToOriginal[model.ID] = model.ID
		}

		// Check if model is in config
		if s.config != nil && s.config.Policies != nil {
			for _, policy := range s.config.Policies {
				if policy.Models != nil {
					// Check for exact model name match (with tag)
					if modelPolicy, ok := policy.Models[model.ID]; ok {
						enhancedModels[i].RateLimit = modelPolicy.RateLimit
						enhancedModels[i].MaxTokens = modelPolicy.MaxTokens
						enhancedModels[i].InConfig = true
					}

					// Also check for wildcard match (e.g., "mistral" matches "mistral:latest")
					// And model-specific configs like "tinyllama:*"
					modelNameBase := extractModelBaseName(model.ID)
					if modelPolicy, ok := policy.Models[modelNameBase]; ok && !enhancedModels[i].InConfig {
						enhancedModels[i].RateLimit = modelPolicy.RateLimit
						enhancedModels[i].MaxTokens = modelPolicy.MaxTokens
						enhancedModels[i].InConfig = true
					}

					// Check for wildcard patterns in models (e.g., "mistral:*", "tinyllama:*")
					for pattern := range policy.Models {
						if isModelMatch(model.ID, pattern) && !enhancedModels[i].InConfig {
							if modelPolicy, ok := policy.Models[pattern]; ok {
								enhancedModels[i].RateLimit = modelPolicy.RateLimit
								enhancedModels[i].MaxTokens = modelPolicy.MaxTokens
								enhancedModels[i].InConfig = true
							}
						}
					}
				}

				// Check if model is in allowed_models list
				for _, allowed := range policy.AllowedModels {
					if isModelMatch(model.ID, allowed) {
						enhancedModels[i].InConfig = true
						enhancedModels[i].RateLimit = policy.RateLimit
						enhancedModels[i].MaxTokens = policy.MaxTokens
					}
				}
			}
		}

		// Check intent priorities
		if s.config != nil && s.config.Policies != nil {
			// Get model name without upstream for intent matching
			modelName := model.ID
			if slashIndex := strings.Index(model.ID, "/"); slashIndex >= 0 {
				modelName = model.ID[slashIndex+1:]
			}

			var intents []string
			for intentName, intent := range s.config.Intents {
				if intent.Priority != nil {
					for _, priorityModel := range intent.Priority {
						// Check exact match or wildcard
						if isModelMatch(modelName, priorityModel) {
							intents = append(intents, intentName)
							break // Only add each intent once
						}
					}
				}
			}
			enhancedModels[i].Intents = intents
		}
	}

	s.writeJSON(w, http.StatusOK, openai.ModelList{
		Object: "list",
		Data:   enhancedModels,
	})
}

// extractModelBaseName extracts the base model name without tag
// Also strips upstream prefix if present (e.g., "default/mistral:latest" -> "mistral")
func extractModelBaseName(modelName string) string {
	// Strip upstream prefix if present
	// e.g., "default/mistral:latest" -> "mistral:latest"
	slashIndex := strings.Index(modelName, "/")
	if slashIndex >= 0 {
		modelName = modelName[slashIndex+1:]
	}

	// Strip tag
	// e.g., "mistral:latest" -> "mistral"
	colonIndex := strings.Index(modelName, ":")
	if colonIndex > 0 {
		modelName = modelName[:colonIndex]
	}
	return modelName
}

// isModelMatch checks if a model name matches an allowed model selector
// Supports exact match, tag wildcards (e.g., "mistral:*", "tinyllama:*"), and glob patterns
// Handles upstream prefix (e.g., "default/mistral" matches "mistral")
func isModelMatch(model, selector string) bool {
	// Strip upstream prefixes for comparison
	strippedModel := model
	if slashIndex := strings.Index(model, "/"); slashIndex >= 0 {
		strippedModel = model[slashIndex+1:]
	}
	strippedSelector := selector
	if slashIndex := strings.Index(selector, "/"); slashIndex >= 0 {
		strippedSelector = selector[slashIndex+1:]
	}

	// Check both with and without upstream prefix
	if strippedModel == strippedSelector || model == selector {
		return true
	}

	// Handle wildcards like "mistral:*" or "tinyllama:*"
	// The wildcard is "NAME:*" where * matches any tag
	if strings.Contains(strippedSelector, ":*") {
		// Split selector into base and check if model starts with base
		parts := strings.SplitN(strippedSelector, ":*", 2)
		if len(parts) == 2 {
			prefix := parts[0]
			return strings.HasPrefix(strippedModel, prefix+":")
		}
	}

	// Also check for just "*" wildcard
	if strings.HasSuffix(strippedSelector, "*") {
		prefix := strings.TrimSuffix(strippedSelector, "*")
		return strings.HasPrefix(strippedModel, prefix)
	}

	return false
}

// handleAdminModelsPull handles POST /admin/api/models/pull
func (s *Server) handleAdminModelsPull(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		ModelName string `json:"model_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ModelName == "" {
		s.writeError(w, http.StatusBadRequest, "model_name is required")
		return
	}

	// Use background context for pull operation (can take a long time)
	// The pull runs asynchronously and progress is checked via polling
	ctx := context.Background()
	progress, _ := s.modelManager.PullModel(ctx, req.ModelName)

	// Always return progress, job ID is enough for frontend to poll
	// Error is handled internally via progress status
	s.writeJSON(w, http.StatusOK, progress)
}

// handleAdminModelsPullProgress handles GET /admin/api/models/pull/{job_id}
func (s *Server) handleAdminModelsPullProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	jobID := strings.TrimPrefix(r.URL.Path, "/admin/api/models/pull/")
	if jobID == "" {
		s.writeError(w, http.StatusBadRequest, "job_id is required")
		return
	}

	progress := s.modelManager.GetPullProgress(jobID)
	if progress == nil {
		s.writeError(w, http.StatusNotFound, "job not found")
		return
	}

	s.writeJSON(w, http.StatusOK, progress)
}

// handleAdminModelsDelete handles DELETE /admin/api/models/{name}
func (s *Server) handleAdminModelsDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract model name from URL: /admin/api/models/{name}
	modelName := strings.TrimPrefix(r.URL.Path, "/admin/api/models/")
	if modelName == "" {
		s.writeError(w, http.StatusBadRequest, "model name is required")
		return
	}

	ctx := r.Context()
	if err := s.modelManager.RemoveModel(ctx, modelName); err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "model": modelName})
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Dashboard is accessible without auth (for convenience)
		if strings.HasPrefix(r.URL.Path, "/admin") {
			next.ServeHTTP(w, r)
			return
		}

		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		if r.URL.Path == "" || r.URL.Path == "/" || r.URL.Path == "/index.html" {
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth == "" {
			s.writeError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		var keyValue string
		if strings.HasPrefix(auth, "Bearer ") {
			keyValue = auth[7:]
		} else {
			keyValue = auth
		}

		key, ok := s.keystore.Get(keyValue)
		if !ok {
			s.writeError(w, http.StatusUnauthorized, "invalid api key")
			return
		}

		s.keystore.RecordUsage(keyValue)

		ctx := context.WithValue(r.Context(), "key", key)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		duration := time.Since(start)

		// Log to console
		fmt.Printf("[REQUEST] %s %s %s - Status: %d (%v)\n", r.Method, r.URL.Path, r.RemoteAddr, rw.statusCode, duration)

		// Broadcast to dashboard
		entry := LogEntry{
			Time:     time.Now().Format("15:04:05"),
			Method:   r.Method,
			Path:     r.URL.Path,
			Status:   rw.statusCode,
			Duration: duration.String(),
		}
		s.logHub.broadcastEntry(entry)
	})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, openai.ErrorResponse{
		Error: openai.APIError{
			Message: message,
			Type:    "api_error",
			Code:    fmt.Sprintf("%d", status),
		},
	})
}

// Async job handlers

// handleAsyncSubmit handles POST /v1/async
func (s *Server) handleAsyncSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req openai.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Create job
	job := s.jobStore.Create(&req)

	// Add to queue
	s.jobQueue.Enqueue(job.ID)

	// Return job ID
	s.writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"job_id": job.ID,
		"status": job.Status,
	})
}

// handleJobStatus handles GET /v1/jobs/{id}
func (s *Server) handleJobStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract job ID from /v1/jobs/{id}
	jobID := strings.TrimPrefix(r.URL.Path, "/v1/jobs/")
	if jobID == "" || strings.Contains(jobID, "/") {
		s.writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}

	// Get job
	job, ok := s.jobStore.Get(jobID)
	if !ok {
		s.writeError(w, http.StatusNotFound, "job not found")
		return
	}

	// Return job status
	s.writeJSON(w, http.StatusOK, job)
}

// Provider handlers

// handleAdminProviders handles GET/POST /admin/api/providers
func (s *Server) handleAdminProviders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if s.providerStore == nil {
			s.writeJSON(w, http.StatusOK, []interface{}{})
			return
		}
		providers, err := s.providerStore.List()
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list providers: %v", err))
			return
		}
		s.writeJSON(w, http.StatusOK, providers)

	case http.MethodPost:
		var req struct {
			ID         string   `json:"id"`
			Name       string   `json:"name"`
			Type       string   `json:"type"`
			APIKey     string   `json:"api_key"`
			BaseURL    string   `json:"base_url"`
			Models     []string `json:"models"`
			RateLimit  string   `json:"rate_limit"`
			Priority   int      `json:"priority"`
			TrackCosts *bool    `json:"track_costs"`
			IsActive   *bool    `json:"is_active"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		id := req.ID
		if id == "" {
			// Generate ID
			id = fmt.Sprintf("%s-%d", req.Type, time.Now().UnixNano())
		} else {
			if !isSafeProviderID(id) {
				s.writeError(w, http.StatusBadRequest, "invalid provider id (use letters, numbers, '-', '_' only)")
				return
			}
			// Prevent collisions with upstream names
			for _, up := range s.upstreams.List() {
				if up == id {
					s.writeError(w, http.StatusBadRequest, "provider id conflicts with an upstream name")
					return
				}
			}
			if s.providerStore != nil {
				if _, err := s.providerStore.Get(id); err == nil {
					s.writeError(w, http.StatusBadRequest, "provider id already exists")
					return
				}
			}
		}

		provider := &providers.ProviderConfig{
			ID:         id,
			Name:       req.Name,
			Type:       req.Type,
			APIKey:     req.APIKey,
			BaseURL:    req.BaseURL,
			Models:     req.Models,
			RateLimit:  req.RateLimit,
			Priority:   req.Priority,
			IsActive:   true,
			TrackCosts: true,
			CreatedAt:  time.Now(),
		}
		// Apply optional flags (default to true)
		if req.TrackCosts != nil {
			provider.TrackCosts = *req.TrackCosts
		}
		if req.IsActive != nil {
			provider.IsActive = *req.IsActive
		}

		// Auto-discover models if not provided
		if len(provider.Models) == 0 {
			p, err := providers.ProviderFromConfig(provider)
			if err == nil {
				ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
				defer cancel()
				models, err := p.ListModels(ctx)
				if err == nil {
					ids := make([]string, 0, len(models))
					for _, m := range models {
						if m.ID != "" {
							ids = append(ids, m.ID)
						}
					}
					provider.Models = ids
				}
			}
		}

		if err := s.providerStore.Create(provider); err != nil {
			s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create provider: %v", err))
			return
		}

		s.writeJSON(w, http.StatusCreated, provider)

	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleAdminProviderDetail handles GET/PUT/DELETE /admin/api/providers/{id}
func (s *Server) handleAdminProviderDetail(w http.ResponseWriter, r *http.Request) {
	// Extract provider ID
	id := strings.TrimPrefix(r.URL.Path, "/admin/api/providers/")
	if id == "" || strings.Contains(id, "/") {
		s.writeError(w, http.StatusBadRequest, "invalid provider id")
		return
	}

	// For modifications (PUT/POST/DELETE), require providerStore
	if r.Method != http.MethodGet && s.providerStore == nil {
		s.writeError(w, http.StatusServiceUnavailable, "provider store not initialized")
		return
	}

	switch r.Method {
	case http.MethodGet:
		if s.providerStore == nil {
			s.writeError(w, http.StatusNotFound, "provider not found")
			return
		}
		provider, err := s.providerStore.Get(id)
		if err != nil {
			s.writeError(w, http.StatusNotFound, "provider not found")
			return
		}
		s.writeJSON(w, http.StatusOK, provider)

	case http.MethodPut:
		var req struct {
			Name       string   `json:"name"`
			Type       string   `json:"type"`
			APIKey     string   `json:"api_key"`
			BaseURL    string   `json:"base_url"`
			Models     []string `json:"models"`
			RateLimit  string   `json:"rate_limit"`
			Priority   int      `json:"priority"`
			TrackCosts bool     `json:"track_costs"`
			IsActive   bool     `json:"is_active"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		provider := &providers.ProviderConfig{
			ID:         id,
			Name:       req.Name,
			Type:       req.Type,
			APIKey:     req.APIKey,
			BaseURL:    req.BaseURL,
			Models:     req.Models,
			RateLimit:  req.RateLimit,
			Priority:   req.Priority,
			TrackCosts: req.TrackCosts,
			IsActive:   req.IsActive,
		}

		// Auto-discover models if not provided
		if len(provider.Models) == 0 {
			p, err := providers.ProviderFromConfig(provider)
			if err == nil {
				ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
				defer cancel()
				models, err := p.ListModels(ctx)
				if err == nil {
					ids := make([]string, 0, len(models))
					for _, m := range models {
						if m.ID != "" {
							ids = append(ids, m.ID)
						}
					}
					provider.Models = ids
				}
			}
		}

		if err := s.providerStore.Update(provider); err != nil {
			s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update provider: %v", err))
			return
		}

		updated, _ := s.providerStore.Get(id)
		s.writeJSON(w, http.StatusOK, updated)

	case http.MethodDelete:
		if err := s.providerStore.Delete(id); err != nil {
			s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete provider: %v", err))
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleAdminProviderTypes handles GET /admin/api/providers/types
func (s *Server) handleAdminProviderTypes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	types := providers.SupportedProviders()
	s.writeJSON(w, http.StatusOK, types)
}

func (s *Server) handleAdminProviderTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Type    string `json:"type"`
		APIKey  string `json:"api_key"`
		BaseURL string `json:"base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Type == "" || req.APIKey == "" {
		s.writeError(w, http.StatusBadRequest, "type and api_key are required")
		return
	}

	provider, err := providers.NewProvider(req.Type, "test", "test", req.APIKey, req.BaseURL)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	models, err := provider.ListModels(ctx)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Return a small preview
	preview := []string{}
	for i := 0; i < len(models) && i < 10; i++ {
		preview = append(preview, models[i].ID)
	}

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"model_count": len(models),
		"models":      preview,
	})
}

func isSafeProviderID(id string) bool {
	if len(id) < 2 || len(id) > 64 {
		return false
	}
	for _, ch := range id {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' {
			continue
		}
		return false
	}
	return true
}

// Addr returns the server address
func (s *Server) Addr() string {
	return s.server.Addr
}

// handleNetwork handles GET /admin/api/network
func (s *Server) handleNetwork(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get all public IP addresses
	type Response struct {
		BindHost  string   `json:"bind_host"`
		Port      int      `json:"port"`
		IPs       []string `json:"ips"`
		DefaultIP string   `json:"default_ip"`
	}

	bindHost := s.config.Server.Host
	port := s.config.Server.Port

	ips := network.GetPublicIPs()
	defaultIP := network.GetDefaultIP()

	response := Response{
		BindHost:  bindHost,
		Port:      port,
		IPs:       ips,
		DefaultIP: defaultIP,
	}

	s.writeJSON(w, http.StatusOK, response)
}

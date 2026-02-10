package server

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/modeltunnel/modeltunnel/internal/config"
	"github.com/modeltunnel/modeltunnel/internal/gateway"
	"github.com/modeltunnel/modeltunnel/internal/jobs"
	"github.com/modeltunnel/modeltunnel/internal/keys"
	"github.com/modeltunnel/modeltunnel/internal/models"
	"github.com/modeltunnel/modeltunnel/internal/router"
	"github.com/modeltunnel/modeltunnel/internal/upstream"
	"github.com/modeltunnel/modeltunnel/pkg/openai"
	"gopkg.in/yaml.v3"
)

//go:embed static/dashboard.html
var dashboardHTML string

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
	modelManager  *models.Manager
}

// TunnelStatus holds the current tunnel connection status
type TunnelStatus struct {
	Connected bool   `json:"connected"`
	URL       string `json:"url,omitempty"`
}

// NewServer creates a new server
func NewServer(cfg *config.Config, upstreams *upstream.Manager, keystore *keys.Store, configPath string) *Server {
	// Initialize job system
	jobStore := jobs.NewStore()
	jobQueue := jobs.NewQueue(1000)
	jobWorkers := jobs.NewWorkerPool(3, jobQueue, jobStore, upstreams)
	jobWorkers.Start()

	s := &Server{
		mux:          http.NewServeMux(),
		upstreams:    upstreams,
		keystore:     keystore,
		config:       cfg,
		intentRouter: router.NewIntentRouterFromConfig(cfg.Intents),
		jobStore:     jobStore,
		jobQueue:     jobQueue,
		jobWorkers:   jobWorkers,
		logHub:       newLogHub(),
		configPath:   configPath,
		modelManager: models.NewManager(""),
	}

	// Setup routes
	s.mux.HandleFunc("/v1/models", s.handleModels)
	s.mux.HandleFunc("/v1/chat/completions", s.handleChatCompletions)
	s.mux.HandleFunc("/v1/async", s.handleAsyncSubmit)
	s.mux.HandleFunc("/v1/jobs/", s.handleJobStatus)
	s.mux.HandleFunc("/health", s.handleHealth)

	// Dashboard routes
	s.mux.HandleFunc("/admin", s.handleDashboard)
	s.mux.HandleFunc("/admin/api/keys", s.handleAdminKeys)
	s.mux.HandleFunc("/admin/api/keys/", s.handleAdminKeyDetail)
	s.mux.HandleFunc("/admin/api/logs", s.handleAdminLogs)
	s.mux.HandleFunc("/admin/api/tunnel", s.handleAdminTunnel)
	s.mux.HandleFunc("/admin/api/config", s.handleAdminConfig)
	s.mux.HandleFunc("/admin/api/models", s.handleAdminModels)
	s.mux.HandleFunc("/admin/api/models/pull", s.handleAdminModelsPull)
	s.mux.HandleFunc("/admin/api/models/pull/", s.handleAdminModelsPullProgress)
	s.mux.HandleFunc("/admin/api/models/", s.handleAdminModelsDelete)

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

	models, err := s.upstreams.AllModels(ctx)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
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
		// Get available models from upstream
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		availableModels, _ := s.upstreams.AllModels(ctx)

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

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
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
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	s.mu.RLock()
	status := s.tunnelStatus
	s.mu.RUnlock()

	if status == nil {
		s.writeJSON(w, http.StatusOK, TunnelStatus{Connected: false})
		return
	}

	s.writeJSON(w, http.StatusOK, status)
}

// SetTunnelStatus updates the tunnel connection status
func (s *Server) SetTunnelStatus(connected bool, url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tunnelStatus = &TunnelStatus{
		Connected: connected,
		URL:       url,
	}
}

func (s *Server) handleAdminConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Read current config file
		data, err := os.ReadFile(s.configPath)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "failed to read config")
			return
		}

		s.writeJSON(w, http.StatusOK, map[string]interface{}{
			"config":      string(data),
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
	for i, model := range models {
		enhancedModels[i] = model

		// Check if model is in config
		if s.config != nil && s.config.Policies != nil {
			for _, policy := range s.config.Policies {
				if policy.Models != nil {
					// Check for exact model name match (with tag)
					if modelPolicy, ok := policy.Models[model.ID]; ok {
						enhancedModels[i].RateLimit = modelPolicy.RateLimit
						enhancedModels[i].InConfig = true
					}

					// Also check for wildcard match (e.g., "mistral" matches "mistral:latest")
					modelNameBase := extractModelBaseName(model.ID)
					if modelPolicy, ok := policy.Models[modelNameBase]; ok && enhancedModels[i].RateLimit == "" {
						enhancedModels[i].RateLimit = modelPolicy.RateLimit
						enhancedModels[i].InConfig = true
					}
				}

				// Check if model is in allowed_models list
				for _, allowed := range policy.AllowedModels {
					if isModelMatch(model.ID, allowed) {
						enhancedModels[i].InConfig = true
						enhancedModels[i].RateLimit = policy.RateLimit
					}
				}
			}
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

// Addr returns the server address
func (s *Server) Addr() string {
	return s.server.Addr
}

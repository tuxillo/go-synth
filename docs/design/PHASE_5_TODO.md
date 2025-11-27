# Phase 5: Minimal REST API - Implementation Tasks

**Phase**: 5 of 7 (Optional)  
**Status**: ⚪ Planned  
**Dependencies**: Phases 1-3 complete (✅), Phase 4 in progress  
**Estimated Effort**: ~14 hours  
**Priority**: Low (Optional feature)

## Overview

Phase 5 adds a minimal REST API to enable remote build automation. This is an **optional** 
feature for users who want to trigger builds programmatically or integrate with CI/CD systems.

### Goals

- **Remote Build Trigger**: POST endpoint to start builds via HTTP
- **Status Polling**: GET endpoints to query build status and list builds  
- **Simple Authentication**: API key-based auth (no OAuth complexity)
- **Integration**: Wire API handlers to existing Builder and BuildDB

### Non-Goals (MVP)

- ❌ WebSocket/SSE streaming (polling only)
- ❌ Multi-user authentication (single API key)
- ❌ Rate limiting or advanced security
- ❌ GraphQL or alternative APIs
- ❌ Web UI/dashboard (API only)

---

## Implementation Tasks

### Task 1: Define API Package Structure ⚪

**Estimated Time**: 1 hour  
**Priority**: High  
**Dependencies**: None

#### Description

Create the basic package structure for the REST API with clean separation of concerns.

#### Implementation Steps

1. **Create package directory**:
   ```bash
   mkdir -p api/v1
   ```

2. **Create `api/types.go`**:
   ```go
   // Package api provides HTTP REST API for build automation.
   package api
   
   import "time"
   
   // BuildRequest represents a request to start a new build
   type BuildRequest struct {
       Packages []string `json:"packages"` // List of port directories
       Profile  string   `json:"profile"`  // Build profile name (default: "default")
       Force    bool     `json:"force"`    // Skip CRC checks (default: false)
   }
   
   // BuildResponse contains the UUID of a newly created build
   type BuildResponse struct {
       BuildID string `json:"build_id"` // UUID of the build
   }
   
   // BuildStatusResponse provides detailed status of a specific build
   type BuildStatusResponse struct {
       BuildID   string    `json:"build_id"`
       Status    string    `json:"status"`     // "running" | "success" | "failed"
       Packages  []string  `json:"packages"`   // Original package list
       Profile   string    `json:"profile"`
       StartTime time.Time `json:"start_time"`
       EndTime   time.Time `json:"end_time,omitempty"` // Only set when complete
       Stats     Stats     `json:"stats"`
   }
   
   // Stats contains build execution statistics
   type Stats struct {
       Total   int `json:"total"`
       Success int `json:"success"`
       Failed  int `json:"failed"`
       Skipped int `json:"skipped"`
   }
   
   // BuildListResponse provides paginated list of all builds
   type BuildListResponse struct {
       Items []BuildListItem `json:"items"`
       Next  string          `json:"next,omitempty"` // Cursor for pagination
   }
   
   // BuildListItem is a summary of a single build
   type BuildListItem struct {
       BuildID   string    `json:"build_id"`
       Status    string    `json:"status"`
       Profile   string    `json:"profile"`
       StartTime time.Time `json:"start_time"`
       EndTime   time.Time `json:"end_time,omitempty"`
   }
   
   // ErrorResponse is returned for all API errors
   type ErrorResponse struct {
       Status  string `json:"status"`  // Always "error"
       Code    string `json:"code"`    // Machine-readable error code
       Message string `json:"message"` // Human-readable message
   }
   ```

3. **Create `api/errors.go`**:
   ```go
   package api
   
   import (
       "encoding/json"
       "net/http"
   )
   
   // Error codes
   const (
       ErrInvalidRequest   = "invalid_request"
       ErrUnauthorized     = "unauthorized"
       ErrNotFound         = "not_found"
       ErrBuildFailed      = "build_failed"
       ErrInternalError    = "internal_error"
   )
   
   // WriteError writes a JSON error response
   func WriteError(w http.ResponseWriter, statusCode int, code, message string) {
       w.Header().Set("Content-Type", "application/json")
       w.WriteHeader(statusCode)
       
       resp := ErrorResponse{
           Status:  "error",
           Code:    code,
           Message: message,
       }
       json.NewEncoder(w).Encode(resp)
   }
   ```

#### Testing Checklist

- [ ] Package imports cleanly
- [ ] All types can be JSON marshaled/unmarshaled
- [ ] Error codes are well-defined constants
- [ ] Documentation is clear and complete

---

### Task 2: Implement API Key Middleware ⚪

**Estimated Time**: 1.5 hours  
**Priority**: High  
**Dependencies**: Task 1

#### Description

Add simple API key authentication via `X-API-Key` header. This prevents unauthorized 
access to build endpoints.

#### Implementation Steps

1. **Update `config/config.go`** to add API configuration:
   ```go
   // In Config struct
   API struct {
       Enabled bool   `json:"api_enabled"`
       Listen  string `json:"api_listen"`  // e.g., ":8080"
       APIKey  string `json:"api_key"`     // SHA256 hash of actual key
   } `json:"api"`
   ```

2. **Create `api/middleware.go`**:
   ```go
   package api
   
   import (
       "crypto/sha256"
       "crypto/subtle"
       "encoding/hex"
       "net/http"
   )
   
   // AuthMiddleware validates API key from X-API-Key header
   func AuthMiddleware(apiKeyHash string) func(http.Handler) http.Handler {
       return func(next http.Handler) http.Handler {
           return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
               // Extract API key from header
               providedKey := r.Header.Get("X-API-Key")
               if providedKey == "" {
                   WriteError(w, http.StatusUnauthorized, ErrUnauthorized,
                       "Missing X-API-Key header")
                   return
               }
               
               // Hash the provided key
               hash := sha256.Sum256([]byte(providedKey))
               providedHash := hex.EncodeToString(hash[:])
               
               // Constant-time comparison to prevent timing attacks
               if subtle.ConstantTimeCompare([]byte(providedHash), []byte(apiKeyHash)) != 1 {
                   WriteError(w, http.StatusUnauthorized, ErrUnauthorized,
                       "Invalid API key")
                   return
               }
               
               // Key valid, proceed to handler
               next.ServeHTTP(w, r)
           })
       }
   }
   
   // LoggingMiddleware logs all API requests
   func LoggingMiddleware(next http.Handler) http.Handler {
       return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
           // TODO: Use proper logger from context
           println("[API]", r.Method, r.URL.Path, r.RemoteAddr)
           next.ServeHTTP(w, r)
       })
   }
   ```

3. **Add key generation utility** in `cmd/dsynth/main.go`:
   ```go
   // In main command parsing
   case "generate-api-key":
       key := make([]byte, 32)
       if _, err := rand.Read(key); err != nil {
           log.Fatal(err)
       }
       apiKey := hex.EncodeToString(key)
       hash := sha256.Sum256([]byte(apiKey))
       hashStr := hex.EncodeToString(hash[:])
       
       fmt.Println("Generated API Key (save this securely):")
       fmt.Println(apiKey)
       fmt.Println("\nAdd this hash to your config.json:")
       fmt.Printf(`"api": { "api_key": "%s" }\n`, hashStr)
   ```

#### Testing Checklist

- [ ] Valid API key allows access
- [ ] Missing API key returns 401 Unauthorized
- [ ] Invalid API key returns 401 Unauthorized
- [ ] Constant-time comparison prevents timing attacks
- [ ] `generate-api-key` command works correctly
- [ ] API key hash is never logged

---

### Task 3: Implement POST /api/v1/builds Handler ⚪

**Estimated Time**: 3 hours  
**Priority**: High  
**Dependencies**: Tasks 1, 2

#### Description

Create endpoint to trigger new builds. This is the core API functionality.

#### Implementation Steps

1. **Create `api/handlers.go`**:
   ```go
   package api
   
   import (
       "context"
       "encoding/json"
       "net/http"
       "sync"
       
       "dsynth/build"
       "dsynth/builddb"
       "dsynth/config"
       "dsynth/log"
       "dsynth/pkg"
       
       "github.com/google/uuid"
   )
   
   // Server holds API state and dependencies
   type Server struct {
       cfg       *config.Config
       logger    *log.Logger
       db        *builddb.DB
       pkgReg    *pkg.PackageRegistry
       stateReg  *pkg.BuildStateRegistry
       
       // Track active builds
       builds   map[string]*activeBuild
       buildsMu sync.RWMutex
   }
   
   // activeBuild tracks an in-progress build
   type activeBuild struct {
       ID        string
       Request   BuildRequest
       Status    string // "running" | "success" | "failed"
       Stats     *build.BuildStats
       Packages  []*pkg.Package
       StartTime time.Time
       EndTime   time.Time
       ctx       context.Context
       cancel    context.CancelFunc
   }
   
   // NewServer creates a new API server
   func NewServer(cfg *config.Config, logger *log.Logger, db *builddb.DB) *Server {
       return &Server{
           cfg:      cfg,
           logger:   logger,
           db:       db,
           pkgReg:   pkg.NewPackageRegistry(),
           stateReg: pkg.NewBuildStateRegistry(),
           builds:   make(map[string]*activeBuild),
       }
   }
   
   // HandleCreateBuild handles POST /api/v1/builds
   func (s *Server) HandleCreateBuild(w http.ResponseWriter, r *http.Request) {
       // Decode request
       var req BuildRequest
       if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
           WriteError(w, http.StatusBadRequest, ErrInvalidRequest,
               "Invalid JSON: "+err.Error())
           return
       }
       
       // Validate request
       if len(req.Packages) == 0 {
           WriteError(w, http.StatusBadRequest, ErrInvalidRequest,
               "At least one package required")
           return
       }
       
       if req.Profile == "" {
           req.Profile = "default"
       }
       
       // Generate build ID
       buildID := uuid.New().String()
       
       // Parse and resolve packages
       packages, err := pkg.ParsePortList(req.Packages, s.cfg, s.stateReg, s.pkgReg)
       if err != nil {
           WriteError(w, http.StatusBadRequest, ErrInvalidRequest,
               "Failed to parse packages: "+err.Error())
           return
       }
       
       if err := pkg.ResolveDependencies(packages, s.cfg, s.stateReg, s.pkgReg); err != nil {
           WriteError(w, http.StatusBadRequest, ErrInvalidRequest,
               "Failed to resolve dependencies: "+err.Error())
           return
       }
       
       // Create build context
       ctx, cancel := context.WithCancel(context.Background())
       ab := &activeBuild{
           ID:        buildID,
           Request:   req,
           Status:    "running",
           Packages:  packages,
           StartTime: time.Now(),
           ctx:       ctx,
           cancel:    cancel,
       }
       
       // Register build
       s.buildsMu.Lock()
       s.builds[buildID] = ab
       s.buildsMu.Unlock()
       
       // Start build in goroutine
       go s.executeBuild(ab)
       
       // Return build ID
       w.Header().Set("Content-Type", "application/json")
       w.WriteHeader(http.StatusCreated)
       json.NewEncoder(w).Encode(BuildResponse{BuildID: buildID})
   }
   
   // executeBuild runs the build in the background
   func (s *Server) executeBuild(ab *activeBuild) {
       stats, cleanup, err := build.DoBuild(ab.Packages, s.cfg, s.logger, s.db)
       defer cleanup()
       
       s.buildsMu.Lock()
       defer s.buildsMu.Unlock()
       
       ab.EndTime = time.Now()
       ab.Stats = stats
       
       if err != nil {
           ab.Status = "failed"
       } else if stats.Failed > 0 {
           ab.Status = "failed"
       } else {
           ab.Status = "success"
       }
   }
   ```

2. **Add force build support**:
   ```go
   // In executeBuild, before DoBuild:
   if ab.Request.Force {
       // TODO: Implement force flag bypass in build package
       // This requires updating build.DoBuild to accept force parameter
   }
   ```

#### Testing Checklist

- [ ] Valid request returns 201 with build_id
- [ ] Missing packages field returns 400
- [ ] Empty packages array returns 400
- [ ] Invalid package names return 400
- [ ] Build executes in background
- [ ] Multiple concurrent builds work correctly
- [ ] Build status tracked properly
- [ ] Force flag bypasses CRC checks

---

### Task 4: Implement GET /api/v1/builds/:id Handler ⚪

**Estimated Time**: 2 hours  
**Priority**: High  
**Dependencies**: Task 3

#### Description

Query status and details of a specific build by UUID.

#### Implementation Steps

1. **Add to `api/handlers.go`**:
   ```go
   // HandleGetBuild handles GET /api/v1/builds/:id
   func (s *Server) HandleGetBuild(w http.ResponseWriter, r *http.Request) {
       // Extract build ID from URL path
       // Assumes router extracts it (see Task 5)
       buildID := r.PathValue("id")
       if buildID == "" {
           WriteError(w, http.StatusBadRequest, ErrInvalidRequest,
               "Missing build ID")
           return
       }
       
       // Find build in active builds
       s.buildsMu.RLock()
       ab, found := s.builds[buildID]
       s.buildsMu.RUnlock()
       
       if !found {
           // Check database for completed builds
           // TODO: Add QueryBuild to builddb package
           WriteError(w, http.StatusNotFound, ErrNotFound,
               "Build not found: "+buildID)
           return
       }
       
       // Build response
       resp := BuildStatusResponse{
           BuildID:   ab.ID,
           Status:    ab.Status,
           Profile:   ab.Request.Profile,
           StartTime: ab.StartTime,
           EndTime:   ab.EndTime,
           Packages:  ab.Request.Packages,
       }
       
       if ab.Stats != nil {
           resp.Stats = Stats{
               Total:   ab.Stats.Total,
               Success: ab.Stats.Success,
               Failed:  ab.Stats.Failed,
               Skipped: ab.Stats.Skipped,
           }
       }
       
       w.Header().Set("Content-Type", "application/json")
       json.NewEncoder(w).Encode(resp)
   }
   ```

2. **Add persistence for completed builds**:
   ```go
   // In executeBuild, after setting status:
   // Persist completed build metadata to database
   // TODO: Extend builddb to store build metadata
   ```

#### Testing Checklist

- [ ] Valid build ID returns 200 with status
- [ ] Invalid build ID returns 404
- [ ] Running build shows correct status
- [ ] Completed build shows stats
- [ ] Failed build shows correct status
- [ ] EndTime only set when build complete

---

### Task 5: Implement GET /api/v1/builds Handler ⚪

**Estimated Time**: 2 hours  
**Priority**: Medium  
**Dependencies**: Task 4

#### Description

List all builds with pagination support.

#### Implementation Steps

1. **Add to `api/handlers.go`**:
   ```go
   // HandleListBuilds handles GET /api/v1/builds
   func (s *Server) HandleListBuilds(w http.ResponseWriter, r *http.Request) {
       // Parse pagination parameters
       limit := 50 // Default limit
       cursor := r.URL.Query().Get("cursor")
       
       // TODO: Implement proper pagination
       // For MVP, just return active builds
       
       s.buildsMu.RLock()
       defer s.buildsMu.RUnlock()
       
       items := make([]BuildListItem, 0, len(s.builds))
       for _, ab := range s.builds {
           items = append(items, BuildListItem{
               BuildID:   ab.ID,
               Status:    ab.Status,
               Profile:   ab.Request.Profile,
               StartTime: ab.StartTime,
               EndTime:   ab.EndTime,
           })
       }
       
       // Sort by start time (most recent first)
       sort.Slice(items, func(i, j int) bool {
           return items[i].StartTime.After(items[j].StartTime)
       })
       
       // Apply limit
       if len(items) > limit {
           items = items[:limit]
           // TODO: Generate next cursor
       }
       
       resp := BuildListResponse{Items: items}
       
       w.Header().Set("Content-Type", "application/json")
       json.NewEncoder(w).Encode(resp)
   }
   ```

#### Testing Checklist

- [ ] Returns list of all active builds
- [ ] Sorted by start time (newest first)
- [ ] Limit parameter works
- [ ] Empty list returns valid JSON
- [ ] Pagination cursor generated (optional for MVP)

---

### Task 6: Add HTTP Router and Server Setup ⚪

**Estimated Time**: 2 hours  
**Priority**: High  
**Dependencies**: Tasks 1-5

#### Description

Wire up all handlers with routing and start HTTP server.

#### Implementation Steps

1. **Create `api/server.go`**:
   ```go
   package api
   
   import (
       "context"
       "fmt"
       "net/http"
       "time"
       
       "dsynth/builddb"
       "dsynth/config"
       "dsynth/log"
   )
   
   // Start starts the API server
   func Start(cfg *config.Config, logger *log.Logger, db *builddb.DB) error {
       if !cfg.API.Enabled {
           return nil
       }
       
       server := NewServer(cfg, logger, db)
       
       // Create router (using standard library ServeMux with Go 1.22+ patterns)
       mux := http.NewServeMux()
       
       // Register routes
       mux.HandleFunc("POST /api/v1/builds", server.HandleCreateBuild)
       mux.HandleFunc("GET /api/v1/builds/{id}", server.HandleGetBuild)
       mux.HandleFunc("GET /api/v1/builds", server.HandleListBuilds)
       
       // Health check endpoint (no auth required)
       mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
           w.WriteHeader(http.StatusOK)
           fmt.Fprint(w, "OK")
       })
       
       // Apply middleware
       var handler http.Handler = mux
       handler = AuthMiddleware(cfg.API.APIKey)(handler)
       handler = LoggingMiddleware(handler)
       
       // Create HTTP server
       srv := &http.Server{
           Addr:         cfg.API.Listen,
           Handler:      handler,
           ReadTimeout:  15 * time.Second,
           WriteTimeout: 15 * time.Second,
           IdleTimeout:  60 * time.Second,
       }
       
       logger.Info("Starting API server on %s", cfg.API.Listen)
       
       // Start in goroutine
       go func() {
           if err := srv.ListenAndServe(); err != http.ErrServerClosed {
               logger.Error("API server failed: %v", err)
           }
       }()
       
       return nil
   }
   ```

2. **Update `cmd/dsynth/main.go`**:
   ```go
   // After opening builddb
   if err := api.Start(cfg, logger, db); err != nil {
       log.Fatalf("Failed to start API server: %v", err)
   }
   ```

#### Testing Checklist

- [ ] Server starts on configured port
- [ ] All routes respond correctly
- [ ] Middleware applied to all protected routes
- [ ] /health endpoint works without auth
- [ ] Graceful shutdown on SIGTERM
- [ ] Concurrent requests handled correctly

---

### Task 7: Add Configuration and Documentation ⚪

**Estimated Time**: 1.5 hours  
**Priority**: Medium  
**Dependencies**: Task 6

#### Description

Document API usage and add configuration examples.

#### Implementation Steps

1. **Create `docs/api/REST_API.md`**:
   ````markdown
   # REST API Reference
   
   ## Authentication
   
   All API requests (except `/health`) require an API key via the `X-API-Key` header:
   
   ```bash
   curl -H "X-API-Key: your-api-key-here" http://localhost:8080/api/v1/builds
   ```
   
   ### Generating an API Key
   
   ```bash
   dsynth generate-api-key
   ```
   
   Add the hash to your `config.json`:
   
   ```json
   {
     "api": {
       "enabled": true,
       "listen": ":8080",
       "api_key": "hash-from-generate-command"
     }
   }
   ```
   
   ## Endpoints
   
   ### POST /api/v1/builds
   
   Start a new build.
   
   **Request**:
   ```json
   {
     "packages": ["editors/vim", "devel/git"],
     "profile": "default",
     "force": false
   }
   ```
   
   **Response** (201 Created):
   ```json
   {
     "build_id": "550e8400-e29b-41d4-a716-446655440000"
   }
   ```
   
   ### GET /api/v1/builds/:id
   
   Get status of a specific build.
   
   **Response** (200 OK):
   ```json
   {
     "build_id": "550e8400-e29b-41d4-a716-446655440000",
     "status": "running",
     "packages": ["editors/vim"],
     "profile": "default",
     "start_time": "2025-11-27T10:00:00Z",
     "end_time": "2025-11-27T10:05:00Z",
     "stats": {
       "total": 5,
       "success": 4,
       "failed": 0,
       "skipped": 1
     }
   }
   ```
   
   ### GET /api/v1/builds
   
   List all builds.
   
   **Response** (200 OK):
   ```json
   {
     "items": [
       {
         "build_id": "550e8400-e29b-41d4-a716-446655440000",
         "status": "success",
         "profile": "default",
         "start_time": "2025-11-27T10:00:00Z",
         "end_time": "2025-11-27T10:05:00Z"
       }
     ],
     "next": ""
   }
   ```
   
   ## Error Responses
   
   All errors return JSON with this format:
   
   ```json
   {
     "status": "error",
     "code": "invalid_request",
     "message": "At least one package required"
   }
   ```
   
   **Error Codes**:
   - `invalid_request`: Malformed request (400)
   - `unauthorized`: Missing or invalid API key (401)
   - `not_found`: Build not found (404)
   - `internal_error`: Server error (500)
   
   ## Examples
   
   ### Start a build
   
   ```bash
   curl -X POST \
     -H "X-API-Key: your-key" \
     -H "Content-Type: application/json" \
     -d '{"packages":["editors/vim"]}' \
     http://localhost:8080/api/v1/builds
   ```
   
   ### Poll build status
   
   ```bash
   BUILD_ID="550e8400-e29b-41d4-a716-446655440000"
   while true; do
     STATUS=$(curl -s -H "X-API-Key: your-key" \
       "http://localhost:8080/api/v1/builds/$BUILD_ID" | jq -r .status)
     echo "Status: $STATUS"
     [[ "$STATUS" != "running" ]] && break
     sleep 5
   done
   ```
   ````

2. **Update `README.md`**:
   ```markdown
   ## REST API (Optional)
   
   go-synth includes an optional REST API for remote build automation.
   
   See [docs/api/REST_API.md](docs/api/REST_API.md) for full documentation.
   
   Quick start:
   ```bash
   # Generate API key
   dsynth generate-api-key
   
   # Enable in config.json
   {
     "api": {
       "enabled": true,
       "listen": ":8080",
       "api_key": "<hash-from-generate-command>"
     }
   }
   
   # Start build via API
   curl -X POST \
     -H "X-API-Key: your-key" \
     -H "Content-Type: application/json" \
     -d '{"packages":["editors/vim"]}' \
     http://localhost:8080/api/v1/builds
   ```
   ```

#### Testing Checklist

- [ ] Documentation is clear and complete
- [ ] All examples work correctly
- [ ] Configuration examples are valid
- [ ] Error codes documented
- [ ] Authentication process explained

---

### Task 8: Integration Tests ⚪

**Estimated Time**: 2 hours  
**Priority**: High  
**Dependencies**: Tasks 1-7

#### Description

Add comprehensive integration tests for the API.

#### Implementation Steps

1. **Create `api/api_test.go`**:
   ```go
   //go:build integration
   // +build integration
   
   package api_test
   
   import (
       "bytes"
       "encoding/json"
       "net/http"
       "net/http/httptest"
       "testing"
       "time"
       
       "dsynth/api"
       "dsynth/builddb"
       "dsynth/config"
       "dsynth/log"
   )
   
   func TestAPIIntegration(t *testing.T) {
       // Setup test environment
       tmpDir := t.TempDir()
       cfg := &config.Config{
           API: config.APIConfig{
               Enabled: true,
               Listen:  ":0",
               APIKey:  "test-key-hash",
           },
       }
       
       logger, _ := log.NewLogger(cfg)
       db, err := builddb.OpenDB(tmpDir + "/test.db")
       if err != nil {
           t.Fatal(err)
       }
       defer db.Close()
       
       server := api.NewServer(cfg, logger, db)
       
       // Test POST /api/v1/builds
       t.Run("CreateBuild", func(t *testing.T) {
           body := api.BuildRequest{
               Packages: []string{"editors/vim"},
               Profile:  "default",
           }
           
           jsonBody, _ := json.Marshal(body)
           req := httptest.NewRequest("POST", "/api/v1/builds", bytes.NewReader(jsonBody))
           req.Header.Set("X-API-Key", "valid-key")
           
           w := httptest.NewRecorder()
           server.HandleCreateBuild(w, req)
           
           if w.Code != http.StatusCreated {
               t.Errorf("Expected 201, got %d", w.Code)
           }
           
           var resp api.BuildResponse
           json.NewDecoder(w.Body).Decode(&resp)
           
           if resp.BuildID == "" {
               t.Error("Expected non-empty build_id")
           }
       })
       
       // Test GET /api/v1/builds/:id
       t.Run("GetBuild", func(t *testing.T) {
           // TODO: Create build first, then query it
       })
       
       // Test authentication
       t.Run("AuthRequired", func(t *testing.T) {
           req := httptest.NewRequest("GET", "/api/v1/builds", nil)
           // No X-API-Key header
           
           w := httptest.NewRecorder()
           // TODO: Test with middleware
           
           if w.Code != http.StatusUnauthorized {
               t.Errorf("Expected 401, got %d", w.Code)
           }
       })
   }
   ```

#### Testing Checklist

- [ ] All endpoints have integration tests
- [ ] Authentication tests pass
- [ ] Error cases covered
- [ ] Concurrent requests tested
- [ ] Tests run with `go test -tags=integration`

---

## Summary

### Estimated Time Breakdown

| Task | Estimated | Critical Path |
|------|-----------|---------------|
| 1. Package Structure | 1h | ✅ |
| 2. Auth Middleware | 1.5h | ✅ |
| 3. POST /builds | 3h | ✅ |
| 4. GET /builds/:id | 2h | ✅ |
| 5. GET /builds | 2h | |
| 6. Server Setup | 2h | ✅ |
| 7. Documentation | 1.5h | |
| 8. Integration Tests | 2h | ✅ |
| **Total** | **15h** | **12h** |

### Exit Criteria

- [ ] POST /api/v1/builds creates and starts builds
- [ ] GET /api/v1/builds/:id returns build status
- [ ] GET /api/v1/builds lists all builds
- [ ] API key authentication works
- [ ] Invalid keys return 401
- [ ] Integration tests pass
- [ ] Documentation complete
- [ ] `generate-api-key` command works

### Dependencies

**Requires**:
- ✅ Phase 1: pkg package (parsing, resolution)
- ✅ Phase 2: builddb package (build tracking)
- ✅ Phase 3: build package (DoBuild)

**Blocks**:
- None (optional feature)

### Code Impact

| Package | New Lines | Changes |
|---------|-----------|---------|
| `api/` (new) | ~800 | Create package |
| `config/` | +10 | Add API config |
| `cmd/dsynth/` | +30 | Add generate-api-key |
| `docs/api/` (new) | ~200 | Documentation |
| **Total** | **~1,040** | **Minimal impact** |

---

## Notes

### Design Decisions

1. **Polling over Streaming**: Use polling instead of WebSocket/SSE for simplicity
2. **Single API Key**: No multi-user auth for MVP
3. **Standard Library Router**: Use Go 1.22+ ServeMux (no external dependencies)
4. **In-Memory Build Tracking**: Active builds in memory, completed in database
5. **SHA256 Hashing**: Store hashed API keys for security

### Future Enhancements (Post-MVP)

- WebSocket streaming for real-time updates
- Multi-user authentication (OAuth2)
- Rate limiting per API key
- Build cancellation endpoint (DELETE /builds/:id)
- Log streaming endpoint
- Metrics endpoint (Prometheus format)
- GraphQL alternative API

### Security Considerations

- ⚠️ API key stored as SHA256 hash in config
- ⚠️ Constant-time comparison prevents timing attacks
- ⚠️ HTTPS strongly recommended for production
- ⚠️ No CORS configuration (API assumed internal)
- ⚠️ No request size limits (should add in production)

### Testing Strategy

- **Unit Tests**: Middleware, error handling, JSON marshaling
- **Integration Tests**: Full request/response cycle, auth, concurrent builds
- **Manual Tests**: curl examples in documentation
- **No E2E Tests**: API is optional, not part of core workflow

---

**Next Phase**: [Phase 6: Testing Strategy](PHASE_6_TODO.md)

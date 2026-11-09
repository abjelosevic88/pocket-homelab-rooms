package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-rooms/internal/store"
)

// Router builds the chi router.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	if s.cfg.TrustProxy {
		r.Use(middleware.RealIP)
	}
	r.Use(middleware.RequestID)
	r.Use(s.logging)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	r.Get("/pocket-ask.js", s.core.AskWidget("rooms"))
	r.Get("/healthz", s.handleHealth)
	r.Get("/readyz", s.handleReady)
	r.Get("/api/health", s.handleHealth)
	if s.cfg.MetricsEnabled {
		r.Get("/metrics", s.handleMetrics)
	}

	r.Route("/api/v1", func(r chi.Router) {
		// the read-only view behind a share link: no account, only the link
		r.Group(func(r chi.Router) {
			r.Use(middleware.Timeout(30 * time.Second))
			r.Get("/public/s/{token}", s.handlePublicPlan)
		})

		// imports, uploads, pictures and AI can take longer than a request may
		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)
			r.Post("/import", s.handleImport)
			r.Post("/photos", s.handleUploadPhoto)
			r.Post("/ai/read-plan", s.handleAIReadPlan)
			r.Post("/ai/sizes", s.handleAISizes)
			r.Post("/ai/tips", s.handleAITips)
			r.Get("/export.json", s.handleExportJSON)
			r.Post("/layouts/{id}/file", s.handleLayoutFile)
			r.Post("/apps/photos/{uuid}/copy", s.handlePhotoCopy)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.Timeout(120 * time.Second))
			r.Post("/auth/login", s.handleLogin)
			r.Post("/auth/setup", s.handleSetup)
			r.Get("/auth/status", s.handleAuthStatus)
			r.Get("/auth/oidc/start", s.handleOIDCStart)
			r.Get("/auth/oidc/callback", s.handleOIDCCallback)

			r.Group(func(r chi.Router) {
				r.Use(s.requireAuth)
				r.Post("/auth/logout", s.handleLogout)
				r.Get("/auth/me", s.handleMe)
				r.Put("/auth/me", s.handleUpdateMe)

				// the overview, and for the rest of the box: Pocket Day, Search, AI
				r.Get("/home", s.handleHome)
				r.Get("/summary", s.handleSummary)
				r.Get("/day/{date}", s.handleDay)
				r.Get("/search", s.handleSearch)
				r.Get("/templates", s.handleTemplates)

				// plans, floors and layouts
				r.Get("/plans", s.handleListPlans)
				r.Post("/plans", s.handleCreatePlan)
				r.Get("/plans/{id}", s.handleGetPlan)
				r.Put("/plans/{id}", s.handleUpdatePlan)
				r.Delete("/plans/{id}", s.handleDeletePlan)
				r.Post("/plans/{id}/share", s.handleSharePlan)
				r.Post("/plans/{id}/duplicate", s.handleDuplicatePlan)
				r.Get("/plans/{id}/summary", s.handlePlanSummary)
				r.Post("/plans/{id}/floors", s.handleCreateFloor)
				r.Put("/floors/{id}", s.handleUpdateFloor)
				r.Delete("/floors/{id}", s.handleDeleteFloor)
				r.Post("/floors/{id}/layouts", s.handleCreateLayout)
				r.Get("/layouts/{id}", s.handleGetLayout)
				r.Put("/layouts/{id}", s.handleUpdateLayout)
				r.Delete("/layouts/{id}", s.handleDeleteLayout)

				// the way in, the checklist, room photos
				r.Get("/plans/{id}/routes", s.handleListRoutes)
				r.Post("/plans/{id}/routes", s.handleCreateRoute)
				r.Put("/routes/{id}", s.handleUpdateRoute)
				r.Delete("/routes/{id}", s.handleDeleteRoute)
				r.Get("/todos", s.handleListTodos)
				r.Post("/todos", s.handleCreateTodo)
				r.Post("/plans/{id}/todos", s.handleCreateTodo)
				r.Put("/todos/{id}", s.handleUpdateTodo)
				r.Delete("/todos/{id}", s.handleDeleteTodo)
				r.Get("/plans/{id}/photos", s.handleListPlanPhotos)
				r.Post("/plans/{id}/photos", s.handleAddPlanPhoto)
				r.Put("/plan-photos/{id}", s.handleUpdatePlanPhoto)
				r.Delete("/plan-photos/{id}", s.handleDeletePlanPhoto)
				r.Get("/photos/{id}", s.handlePhoto)

				// furniture and "will it fit?"
				r.Get("/pieces", s.handleListPieces)
				r.Post("/pieces", s.handleCreatePiece)
				r.Get("/pieces/{id}", s.handleGetPiece)
				r.Put("/pieces/{id}", s.handleUpdatePiece)
				r.Delete("/pieces/{id}", s.handleDeletePiece)
				r.Post("/pieces/{id}/wish", s.handlePieceWish)
				r.Post("/pieces/{id}/bought", s.handlePieceBought)
				r.Post("/fit", s.handleFit)
				r.Get("/fit", s.handleFitQuery)

				// the other Pocket apps
				r.Get("/apps", s.handleApps)
				r.Get("/apps/homes", s.handleHomes)
				r.Get("/apps/home-things", s.handleHomeThings)
				r.Get("/apps/inventory", s.handleInventory)
				r.Get("/apps/wishes", s.handleWishes)
				r.Get("/apps/money", s.handleMoneyAccounts)
				r.Get("/apps/photos", s.handlePhotosList)
				r.Get("/apps/photos/{uuid}/image", s.handlePhotoImage)
				r.Get("/apps/rentals", s.handleRentals)
				r.Get("/apps/chats", s.handleChats)
				r.Post("/apps/chat", s.handleShareChat)
				r.Post("/plans/{id}/note", s.handlePlanNote)
				r.Post("/plans/{id}/tasks", s.handlePlanTasks)

				// AI
				r.Get("/ai/status", s.handleAIStatus)
				r.Get("/ai/models", s.handleAIModels)

				r.Post("/integrations/test", s.handleIntegrationTest)
				r.Post("/notify/test", s.handleNotifyTest)

				r.Get("/users", s.handleListUsers)
				r.Post("/users", s.handleCreateUser)
				r.Put("/users/{id}", s.handleUpdateUser)
				r.Delete("/users/{id}", s.handleDeleteUser)

				r.Get("/settings", s.handleGetSettings)
				r.Put("/settings", s.handlePutSettings)
				r.Get("/settings/system", s.handleSystemInfo)

				r.Get("/tokens", s.handleListTokens)
				r.Post("/tokens", s.handleCreateToken)
				r.Delete("/tokens/{id}", s.handleDeleteToken)

				r.Get("/backup", s.handleBackup)
				r.Get("/export.csv", s.handleExportCSV)
			})
		})
	})

	r.NotFound(s.handleSPA)
	return r
}

// ---------- helpers ----------

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		s.requests.Add(1)
		if ww.Status() >= 500 {
			s.errors.Add(1)
		}
		if strings.HasPrefix(r.URL.Path, "/assets/") {
			return
		}
		s.log.Debug("http", "method", r.Method, "path", r.URL.Path, "status", ww.Status(), "dur", time.Since(start).String(), "ip", r.RemoteAddr)
	})
}

type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{Error: msg})
}

func (s *Server) fail(w http.ResponseWriter, err error, context string) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, http.StatusNotFound, "not found")
		return
	case errors.Is(err, store.ErrConflict), errors.Is(err, core.ErrOperationConflict):
		writeErr(w, http.StatusConflict, "already exists")
		return
	}
	s.log.Error(context, "err", err)
	writeErr(w, http.StatusInternalServerError, context+": "+err.Error())
}

func (s *Server) readBody(r *http.Request) ([]byte, error) {
	return io.ReadAll(http.MaxBytesReader(nil, r.Body, s.cfg.MaxBodyBytes))
}

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 32<<20))
	return dec.Decode(v)
}

// decodeKeys decodes JSON into v and also reports which top-level keys were
// present, so PUT handlers can tell "not sent" from "sent empty".
func decodeKeys(r *http.Request, v any) (map[string]bool, error) {
	b, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	keys := make(map[string]bool, len(raw))
	for k := range raw {
		keys[k] = true
	}
	return keys, json.Unmarshal(b, v)
}

func idParam(r *http.Request, name string) int64 {
	n, _ := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	return n
}

func queryInt(r *http.Request, name string) int {
	n, _ := strconv.Atoi(r.URL.Query().Get(name))
	return n
}

func queryBool(r *http.Request, name string) bool {
	v := strings.ToLower(r.URL.Query().Get(name))
	return v == "1" || v == "true" || v == "yes"
}

func validDate(s string) bool {
	if s == "" {
		return true
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// ---------- health / metrics ----------

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"core_outbox": s.coreOutboxHealth(r.Context()), "status": "ok", "version": Version, "uptime": time.Since(s.started).Round(time.Second).String()})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DB.PingContext(r.Context()); err != nil {
		writeErr(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	counts := s.store.Counts(ctx)
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintf(w, "# HELP pocket_rooms_info Build info.\n# TYPE pocket_rooms_info gauge\npocket_rooms_info{version=%q} 1\n", Version)
	fmt.Fprintf(w, "# HELP pocket_rooms_uptime_seconds Process uptime.\n# TYPE pocket_rooms_uptime_seconds gauge\npocket_rooms_uptime_seconds %d\n", int(time.Since(s.started).Seconds()))
	fmt.Fprintf(w, "# HELP pocket_rooms_http_requests_total HTTP requests served.\n# TYPE pocket_rooms_http_requests_total counter\npocket_rooms_http_requests_total %d\n", s.requests.Load())
	fmt.Fprintf(w, "# HELP pocket_rooms_http_errors_total HTTP 5xx responses.\n# TYPE pocket_rooms_http_errors_total counter\npocket_rooms_http_errors_total %d\n", s.errors.Load())
	fmt.Fprintf(w, "# HELP pocket_rooms_rows Rows per table.\n# TYPE pocket_rooms_rows gauge\n")
	for k, v := range counts {
		fmt.Fprintf(w, "pocket_rooms_rows{table=%q} %d\n", k, v)
	}
}

// ---------- static / SPA ----------

func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if s.webFS == nil {
		http.Error(w, "web UI not built (run `make web`)", http.StatusNotFound)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if f, err := s.webFS.Open(path); err == nil {
		f.Close()
		if strings.HasPrefix(path, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		http.FileServerFS(s.webFS).ServeHTTP(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	r.URL.Path = "/"
	http.FileServerFS(s.webFS).ServeHTTP(w, r)
}

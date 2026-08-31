package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"aegion-dynamic/apismith/auth"
	"aegion-dynamic/apismith/environments"
	"aegion-dynamic/apismith/openapi"
	"aegion-dynamic/apismith/request"
	"aegion-dynamic/apismith/ui"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server is the local API Test Console process.
type Server struct {
	cfg     *environments.Config
	catalog *openapi.Catalog
	auth    *auth.Manager
	exec    *request.Executor
	history *request.History
}

// New constructs a console server around loaded config and catalog.
func New(cfg *environments.Config, catalog *openapi.Catalog) *Server {
	return &Server{
		cfg:     cfg,
		catalog: catalog,
		auth:    auth.NewManager(),
		exec:    request.NewExecutor(),
		history: request.NewHistory(80),
	}
}

// Handler returns the HTTP handler serving the UI and console API.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(requestLogger)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", s.handleHealth)
		r.Get("/catalog", s.handleCatalog)
		r.Get("/environments", s.handleEnvironments)
		r.Get("/auth/status", s.handleAuthStatus)
		r.Post("/auth/jwt", s.handleGenerateJWT)
		r.Post("/auth/clear", s.handleClearAuth)
		r.Post("/request", s.handleExecute)
		r.Get("/history", s.handleHistory)
		r.Get("/history/{id}", s.handleHistoryItem)
		r.Post("/history/clear", s.handleHistoryClear)
	})

	uiFS, err := fs.Sub(ui.Files, ".")
	if err != nil {
		panic(err)
	}
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		serveUIFile(w, "index.html")
	})
	r.Get("/index.html", func(w http.ResponseWriter, r *http.Request) {
		serveUIFile(w, "index.html")
	})
	fileServer := http.FileServer(http.FS(uiFS))
	r.Handle("/styles.css", fileServer)
	r.Handle("/app.js", fileServer)
	return r
}

func serveUIFile(w http.ResponseWriter, name string) {
	b, err := ui.Files.ReadFile(name)
	if err != nil {
		http.Error(w, "ui asset missing", http.StatusInternalServerError)
		return
	}
	switch {
	case strings.HasSuffix(name, ".html"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case strings.HasSuffix(name, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case strings.HasSuffix(name, ".js"):
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	}
	_, _ = w.Write(b)
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s %d %dms", r.Method, r.URL.Path, ww.Status(), time.Since(start).Milliseconds())
		}
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"title":       s.catalog.Title,
		"version":     s.catalog.Version,
		"spec_path":   s.catalog.SpecPath,
		"endpoints":   len(s.catalog.Endpoints),
		"default_env": s.cfg.DefaultEnvironment,
	})
}

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.catalog)
}

func (s *Server) handleEnvironments(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"default":      s.cfg.DefaultEnvironment,
		"environments": s.cfg.Public(),
	})
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.auth.Snapshot())
}

type jwtRequest struct {
	Environment string `json:"environment"`
	ClientID    string `json:"client_id"`
	Username    string `json:"username"`
	Password    string `json:"password"`
}

func (s *Server) handleGenerateJWT(w http.ResponseWriter, r *http.Request) {
	var in jwtRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil && err.Error() != "EOF" {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if in.Environment == "" {
		in.Environment = s.cfg.DefaultEnvironment
	}
	cfg, err := s.cfg.CognitoConfigFor(in.Environment, in.ClientID, in.Username, in.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tokens, err := s.auth.Generate(r.Context(), cfg)
	if err != nil {
		log.Printf("jwt generate failed env=%s client=%s: %v", in.Environment, cfg.ClientID, err)
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": tokens.AccessToken,
		"id_token":     tokens.IDToken,
		"token_type":   tokens.TokenType,
		"expires_in":   tokens.ExpiresIn,
		"client_id":    cfg.ClientID,
		"status":       s.auth.Snapshot(),
	})
}

func (s *Server) handleClearAuth(w http.ResponseWriter, r *http.Request) {
	s.auth.Clear()
	writeJSON(w, http.StatusOK, s.auth.Snapshot())
}

func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	var in request.ExecuteInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if in.Environment == "" {
		in.Environment = s.cfg.DefaultEnvironment
	}
	if in.Method == "" || in.Path == "" {
		writeError(w, http.StatusBadRequest, "method and path are required")
		return
	}
	env := s.cfg.Find(in.Environment)
	if env == nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown environment %q", in.Environment))
		return
	}
	if in.AuthMode == "" {
		in.AuthMode = request.AuthJWT
	}
	if in.AuthMode == request.AuthJWT && in.JWT == "" {
		if tok, ok := s.auth.AccessToken(); ok {
			in.JWT = tok
		}
	}

	out := s.exec.Execute(in, env.BaseURL, env.Production)
	if strings.Contains(strings.ToLower(out.ContentType), "json") {
		out.Body = request.PrettyJSON(out.Body)
	}

	item := s.history.Add(request.HistoryItem{
		Environment: in.Environment,
		Method:      strings.ToUpper(in.Method),
		Path:        in.Path,
		PathParams:  in.PathParams,
		Query:       in.Query,
		Headers:     out.RequestMasked,
		Body:        in.Body,
		AuthMode:    in.AuthMode,
		Status:      out.Status,
		DurationMS:  out.DurationMS,
		OK:          out.OK && out.Error == "",
		Error:       out.Error,
	})

	method := strings.ToUpper(in.Method)
	log.Printf("%s %s %s -> %d %dms", in.Environment, method, in.Path, out.Status, out.DurationMS)

	writeJSON(w, http.StatusOK, map[string]any{
		"response": out,
		"history":  item,
	})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.history.List())
}

func (s *Server) handleHistoryItem(w http.ResponseWriter, r *http.Request) {
	var id int
	if _, err := fmt.Sscanf(chi.URLParam(r, "id"), "%d", &id); err != nil {
		writeError(w, http.StatusBadRequest, "invalid history id")
		return
	}
	item, ok := s.history.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "history item not found")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleHistoryClear(w http.ResponseWriter, r *http.Request) {
	s.history.Clear()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

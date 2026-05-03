package server

import (
	"net/http"
	"time"

	"github.com/jkmpod/sendgrid-mailer/config"
	"github.com/jkmpod/sendgrid-mailer/mailer"
	"github.com/jkmpod/sendgrid-mailer/server/handlers"
)

// Server holds the application dependencies and the HTTP route multiplexer.
type Server struct {
	mailer *mailer.Emailer
	config *config.Config
	mux    *http.ServeMux
}

// NewServer creates a Server and wires up all HTTP routes.
func NewServer(cfg *config.Config) *Server {
	e := mailer.NewEmailer(cfg)

	srv := &Server{
		mailer: e,
		config: cfg,
		mux:    http.NewServeMux(),
	}

	srv.mux.HandleFunc("GET /", srv.handleIndex)
	srv.mux.HandleFunc("POST /upload", handlers.HandleUpload)
	srv.mux.HandleFunc("POST /send", handlers.HandleSend(e, cfg))
	srv.mux.HandleFunc("GET /logs", handlers.HandleLogs(cfg.APIKey))
	srv.mux.HandleFunc("GET /compose", handlers.HandleCompose)
	srv.mux.HandleFunc("GET /config", handlers.HandleConfig(cfg))
	srv.mux.HandleFunc("POST /config", handlers.HandleConfigUpdate(e, cfg))

	return srv
}

// Start begins listening for HTTP requests on the given address.
func (s *Server) Start(addr string) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      0, // Disabled for SSE streaming handlers
	}
	return srv.ListenAndServe()
}

// handleIndex serves the main HTML page.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/index.html")
}

package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
)

// Server represents the webhook HTTP server
type Server struct {
	provider provider.Provider
	router   *chi.Mux
	server   *http.Server
}

// NewServer creates a new webhook server
func NewServer(provider provider.Provider, addr string) *Server {
	s := &Server{
		provider: provider,
		router:   chi.NewRouter(),
	}

	s.setupRoutes()

	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// setupRoutes configures the HTTP routes
func (s *Server) setupRoutes() {
	// Middleware
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(middleware.Timeout(60 * time.Second))

	// Health checks
	s.router.Get("/healthz", s.handleHealthz)
	s.router.Get("/readyz", s.handleReadyz)
	s.router.Get("/", s.handleRoot)

	// DNS provider endpoints
	s.router.Get("/records", s.handleGetRecords)
	s.router.Post("/records", s.handleApplyChanges)
	s.router.Post("/adjustendpoints", s.handleAdjustEndpoints)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	log.Infof("Starting webhook server on %s", s.server.Addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	log.Info("Shutting down webhook server...")
	return s.server.Shutdown(ctx)
}

// handleRoot handles the root endpoint - returns domain filter for external-dns negotiation
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	// Handle content negotiation based on Accept header
	// external-dns sends Accept header for negotiation
	acceptHeader := r.Header.Get("Accept")

	// Get the domain filter from provider
	domainFilter := s.provider.GetDomainFilter()

	// Respond with appropriate content type based on Accept header
	// For negotiation, external-dns expects the DomainFilter serialization
	if acceptHeader != "" {
		// Match the accept header content type
		w.Header().Set("Content-Type", acceptHeader)
	} else {
		w.Header().Set("Content-Type", "application/external.dns.webhook+json;version=1")
	}

	w.WriteHeader(http.StatusOK)

	// Serialize the DomainFilter
	if err := json.NewEncoder(w).Encode(domainFilter); err != nil {
		log.Errorf("Failed to encode domain filter: %v", err)
	}
}

// handleHealthz handles the health check endpoint
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

// handleReadyz handles the readiness check endpoint
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// handleGetRecords handles the GET /records endpoint
func (s *Server) handleGetRecords(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	records, err := s.provider.Records(ctx)
	if err != nil {
		log.Errorf("Failed to get records: %v", err)
		s.respondError(w, http.StatusInternalServerError, "Failed to get records", err)
		return
	}

	s.respondJSON(w, http.StatusOK, records)
}

// handleApplyChanges handles the POST /records endpoint
func (s *Server) handleApplyChanges(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var changes plan.Changes
	if err := json.NewDecoder(r.Body).Decode(&changes); err != nil {
		log.Errorf("Failed to decode changes: %v", err)
		s.respondError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	log.Debugf("Received changes: Create=%d, UpdateOld=%d, UpdateNew=%d, Delete=%d",
		len(changes.Create), len(changes.UpdateOld), len(changes.UpdateNew), len(changes.Delete))
	for i := range changes.Create {
		log.Debugf("--------------------------------------------")
		log.Debugf("Create: %v", changes.Create[i])
	}
	for i := range changes.UpdateOld {
		log.Debugf("--------------------------------------------")
		log.Debugf("UpdateOld: %v", changes.UpdateOld[i])
	}
	for i := range changes.UpdateNew {
		log.Debugf("--------------------------------------------")
		log.Debugf("UpdateNew: %v", changes.UpdateNew[i])
	}
	for i := range changes.Delete {
		log.Debugf("--------------------------------------------")
		log.Debugf("Delete: %v", changes.Delete[i])
	}

	log.Infof("Applying changes: Create=%d, UpdateOld=%d, UpdateNew=%d, Delete=%d",
		len(changes.Create), len(changes.UpdateOld), len(changes.UpdateNew), len(changes.Delete))

	if err := s.provider.ApplyChanges(ctx, &changes); err != nil {
		log.Errorf("Failed to apply changes: %v", err)
		s.respondError(w, http.StatusInternalServerError, "Failed to apply changes", err)
		return
	}

	s.respondJSON(w, http.StatusNoContent, nil)
}

// handleAdjustEndpoints handles the POST /adjustendpoints endpoint
func (s *Server) handleAdjustEndpoints(w http.ResponseWriter, r *http.Request) {
	var endpoints []*endpoint.Endpoint
	if err := json.NewDecoder(r.Body).Decode(&endpoints); err != nil {
		log.Errorf("Failed to decode endpoints: %v", err)
		s.respondError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	adjusted, err := s.provider.AdjustEndpoints(endpoints)
	if err != nil {
		log.Errorf("Failed to adjust endpoints: %v", err)
		s.respondError(w, http.StatusInternalServerError, "Failed to adjust endpoints", err)
		return
	}

	s.respondJSON(w, http.StatusOK, adjusted)
}

// respondJSON responds with JSON
func (s *Server) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Errorf("Failed to encode response: %v", err)
		}
	}
}

// respondError responds with an error
func (s *Server) respondError(w http.ResponseWriter, status int, message string, err error) {
	response := map[string]interface{}{
		"error":   message,
		"details": err.Error(),
	}
	s.respondJSON(w, status, response)
}

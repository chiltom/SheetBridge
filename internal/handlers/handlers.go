package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/chiltom/SheetBridge/internal/models"
	"github.com/chiltom/SheetBridge/internal/services"
	"github.com/chiltom/SheetBridge/internal/utils"
)

// Server represents the server that will handle requests and responses
type Server struct {
	cfg    utils.Config
	svc    services.Service
	server *http.Server
}

// NewServer returns a new Server
func NewServer(cfg utils.Config, svc services.Service) *Server {
	mux := http.NewServeMux()
	srv := &Server{
		cfg: cfg,
		svc: svc,
		server: &http.Server{
			Addr:    cfg.Server.Port,
			Handler: mux,
		},
	}

	mux.HandleFunc("/upload", srv.handleUpload)
	mux.HandleFunc("/commit", srv.handleCommit)
	mux.Handle("/", http.FileServer(http.Dir("web")))

	return srv
}

// Run runs the server and gracefully handles errors when they arise
func (s *Server) Run(ctx context.Context) error {
	/* NOTE: The server is currently served without TLS certificates. To implement
	this, you must provide the TLS certificate file and key file location and
	use the server.ListenAndServeTLS method */
	log.Printf("Starting server on %s", s.cfg.Server.Port)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server failed: %w", err)
	}
	return nil
}

// handleUpload handles CSV upload and preview
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("csvfile")
	if err != nil {
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := s.svc.ParseCSV(file, 50)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error parsing CSV: %v", err), http.StatusBadRequest)
		return
	}

	tableNames, err := s.svc.GetAllTables(r.Context())
	if err != nil {
		http.Error(w, "Error fetching table names", http.StatusInternalServerError)
		return
	}

	response := models.UploadResponse{
		CSVData:    data,
		TableNames: tableNames,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
	}
}

// handleCommit handles data commital requests
func (s *Server) handleCommit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	var req models.CommitRequest
	commitData := r.FormValue("commitData")
	if err := json.Unmarshal([]byte(commitData), &req); err != nil {
		http.Error(w, "Error decoding request", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("csvfile")
	if err != nil {
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if err := s.svc.CommitCSV(r.Context(), req, file); err != nil {
		http.Error(w, fmt.Sprintf("Error committing CSV: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Data committed successfully")
}

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
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
	tmpl   *template.Template
}

// sendJSONError writes a JSON error response
func sendJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// sendJSONSuccess writes a JSON success response
func sendJSONSuccess(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// NewServer returns a new Server
func NewServer(cfg utils.Config, svc services.Service) *Server {
	mux := http.NewServeMux()
	tmpl, err := template.ParseFiles("web/templates/index.html")
	if err != nil {
		log.Fatalf("Error parsing index.html template: %v", err)
	}

	srv := &Server{
		cfg: cfg,
		svc: svc,
		server: &http.Server{
			Addr:    cfg.Server.Port,
			Handler: mux,
		},
		tmpl: tmpl,
	}

	staticFileServer := http.StripPrefix("/static/", http.FileServer(http.Dir("web/static")))
	mux.Handle("/static/", staticFileServer)
	mux.HandleFunc("/", srv.handleIndex)
	mux.HandleFunc("/upload", srv.handleUpload)
	mux.HandleFunc("/commit", srv.handleCommit)
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

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tableNames, err := s.svc.GetAllTables(r.Context())
	if err != nil {
		log.Printf("Error fetching table names for index: %v", err)
		tableNames = []string{} // Default to empty list on error
	}
	pageData := struct{ TableNames []string }{TableNames: tableNames}
	if err := s.tmpl.Execute(w, pageData); err != nil {
		log.Printf("Error executing index.html template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleUpload handles CSV upload and preview
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		sendJSONError(w, "Error parsing form: "+err.Error(), http.StatusBadRequest)
		return
	}
	file, _, err := r.FormFile("csvfile")
	if err != nil {
		sendJSONError(w, "Error retrieving file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := s.svc.ParseCSVForPreview(file, 50)
	if err != nil {
		sendJSONError(w, fmt.Sprintf("Error parsing CSV for preview: %v", err), http.StatusBadRequest)
		return
	}
	tableNames, err := s.svc.GetAllTables(r.Context())
	if err != nil {
		sendJSONError(w, "Error fetching table names: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := models.UploadResponse{CSVData: data, TableNames: tableNames}
	sendJSONSuccess(w, response, http.StatusOK)
}

// handleCommit handles data commital requests
func (s *Server) handleCommit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB limit
		sendJSONError(w, "Error parsing form: "+err.Error(), http.StatusBadRequest)
		return
	}
	var req models.CommitRequest
	commitDataJSON := r.FormValue("commitData")
	if err := json.Unmarshal([]byte(commitDataJSON), &req); err != nil {
		http.Error(w, "Error decoding commit data: "+err.Error(), http.StatusBadRequest)
		return
	}
	fileHeader, _, err := r.FormFile("csvfile")
	if err != nil {
		sendJSONError(w, "Error retrieving CSV file for commit: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer fileHeader.Close()

	if err := s.svc.CommitCSV(r.Context(), req, fileHeader); err != nil {
		sendJSONError(w, fmt.Sprintf("Error committing CSV: %v", err), http.StatusInternalServerError)
		return
	}
	sendJSONSuccess(w, map[string]string{"message": "Data committed successfully"}, http.StatusOK)
}

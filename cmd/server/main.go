package main

import (
	"context"
	"errors" // Standard errors package
	"fmt"
	"html/template"
	stdlog "log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/chiltom/SheetBridge/internal/apperrors"
	"github.com/chiltom/SheetBridge/internal/handlers"
	"github.com/chiltom/SheetBridge/internal/logger"
	"github.com/chiltom/SheetBridge/internal/models"
	"github.com/chiltom/SheetBridge/internal/repositories"
	"github.com/chiltom/SheetBridge/internal/services"
	"github.com/chiltom/SheetBridge/internal/utils"
	// For embedded static/template FS
)

type application struct {
	config        *utils.Config
	logger        *logger.Logger
	templateCache map[string]*template.Template
	repo          *repositories.DBRepository
	csvService    *services.CSVService
	handlers      *handlers.AppHandlers
}

func main() {
	// Load config: pass path to .env if not in root, or let it default
	cfg := utils.LoadConfig() // Tries to load .env from current dir by default
	appLogger := logger.NewStdLogger()

	repo, err := repositories.NewDBRepository(cfg)
	if err != nil {
		appLogger.Errorf("Failed to connect to database: %v", err)
		os.Exit(1)
	}
	defer repo.Close()
	appLogger.Info("Database connection pool established.")

	templateCache, err := newTemplateCache()
	if err != nil {
		appLogger.Errorf("Failed to build template cache: %v", err)
		os.Exit(1)
	}
	appLogger.Info("Template cache built.")

	app := &application{
		config:        cfg,
		logger:        appLogger,
		templateCache: templateCache,
		repo:          repo,
		csvService:    services.NewCSVService(),
	}
	// Pass 'app' as the Renderer to AppHandlers
	app.handlers = handlers.NewAppHandlers(appLogger, app.csvService, app.repo, app)

	// Setup static file server with fs.Sub
	handler, err := app.routes()
	if err != nil {
		appLogger.Errorf("Failed to create FS and routes for static files: %v", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.App.Port),
		Handler:      handler,
		ErrorLog:     stdlog.New(appLogger.ErrorOutput(), "", 0),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		s := <-quit
		appLogger.Infof("Caught signal %s. Shutting down server...", s)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			appLogger.Errorf("Server shutdown failed: %v", err)
		}
		appLogger.Info("Server exited gracefully")
	}()

	appLogger.Infof("Starting server on port %s (Env: %s, DevMode: %t)", cfg.App.Port, cfg.App.Env, cfg.IsDevelopment())
	err = srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		appLogger.Errorf("Server failed to start or unexpectedly closed: %v", err)
		os.Exit(1)
	}
}

// Implement Renderer interface methods on *application
func (app *application) Render(w http.ResponseWriter, r *http.Request, status int, page string, data *models.TemplateData) {
	app.render(w, r, status, page, data)
}

func (app *application) ServerError(w http.ResponseWriter, r *http.Request, err error) {
	app.serverError(w, r, err)
}

func (app *application) ClientError(w http.ResponseWriter, r *http.Request, status int, message string) {
	app.clientError(w, r, status, message)
}

func (app *application) NotFound(w http.ResponseWriter, r *http.Request) {
	app.notFound(w, r)
}

func (app *application) MethodNotAllowed(w http.ResponseWriter, r *http.Request, allowedMethods ...string) {
	app.methodNotAllowed(w, r, allowedMethods...)
}

func (app *application) NewTemplateData(r *http.Request) *models.TemplateData {
	return app.newTemplateData(r)
}

// Error handling helpers
func (app *application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	app.logger.Error(fmt.Errorf("%s %s: %w", r.Method, r.URL.Path, err))
	// Check if it's an AppError to provide a slightly better message
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		http.Error(w, fmt.Sprintf("Internal Server Error: %s (Code: %s)", ae.Message, ae.Code), http.StatusInternalServerError)
	} else {
		http.Error(w, "Internal Server Error: We are sorry, but something went wrong.", http.StatusInternalServerError)
	}
}

func (app *application) clientError(w http.ResponseWriter, r *http.Request, status int, message string) {
	if status != http.StatusNotFound { // Avoid logging every 404 unless verbose debugging is on
		app.logger.Infof("Client error: %s %s - Status %d - Message: %s", r.Method, r.URL.Path, status, message)
	}
	http.Error(w, message, status)
}

func (app *application) notFound(w http.ResponseWriter, r *http.Request) {
	app.clientError(w, r, http.StatusNotFound, "404 Not Found: The page you are looking for does not exist.")
}

func (app *application) methodNotAllowed(w http.ResponseWriter, r *http.Request, allowedMethods ...string) {
	w.Header().Set("Allow", strings.Join(allowedMethods, ", "))
	app.clientError(w, r, http.StatusMethodNotAllowed, fmt.Sprintf("405 Method Not Allowed: This resource only supports %s.", strings.Join(allowedMethods, ", ")))
}

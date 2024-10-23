package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Types for request/response structures
type ArtifactEvent struct {
	SessionID string  `json:"sessionId"`
	Source    string  `json:"source"`
	Event     string  `json:"event"`
	Hash      string  `json:"hash"`
	Duration  float64 `json:"duration,omitempty"`
}

type StatusResponse struct {
	Status string `json:"status"`
}

type UploadResponse struct {
	URLs []string `json:"urls"`
}

type ArtifactInfo struct {
	Size           int     `json:"size,omitempty"`
	TaskDurationMs float64 `json:"taskDurationMs,omitempty"`
	Tag            string  `json:"tag,omitempty"`
	Error          *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type ArtifactQueryRequest struct {
	Hashes []string `json:"hashes"`
}

// FileSystemStorage implements artifact storage using the local filesystem
type FileSystemStorage struct {
	basePath string
}

func NewFileSystemStorage(basePath string) (*FileSystemStorage, error) {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}
	return &FileSystemStorage{basePath: basePath}, nil
}

func (fs *FileSystemStorage) Store(hash string, data io.Reader) error {
	path := filepath.Join(fs.basePath, hash)
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, data); err != nil {
		os.Remove(path) // Clean up on error
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}

func (fs *FileSystemStorage) Get(hash string) (io.ReadCloser, int64, error) {
	path := filepath.Join(fs.basePath, hash)
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, fmt.Errorf("artifact not found")
		}
		return nil, 0, fmt.Errorf("failed to open file: %w", err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, 0, fmt.Errorf("failed to get file info: %w", err)
	}

	return file, info.Size(), nil
}

func (fs *FileSystemStorage) Exists(hash string) (bool, error) {
	path := filepath.Join(fs.basePath, hash)
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Server struct to hold dependencies
type Server struct {
	storage *FileSystemStorage
	logger  *log.Logger
	token   string
}

// Custom logging middleware
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func main() {
	fmt.Println("Starting server...")
	// Get configuration from environment variables
	storagePath := os.Getenv("TURBO_CACHE_DIR")
	if storagePath == "" {
		storagePath = "./turbo-cache" // Default path
	}

	authToken := os.Getenv("TURBO_AUTH_TOKEN")
	if authToken == "" {
		log.Fatal("TURBO_AUTH_TOKEN environment variable is required")
	}

	logPath := os.Getenv("TURBO_LOG_FILE")
	var logger *log.Logger
	if logPath != "" {
		logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal("Failed to open log file:", err)
		}
		logger = log.New(logFile, "", log.LstdFlags)
	} else {
		logger = log.New(os.Stdout, "", log.LstdFlags)
	}

	storage, err := NewFileSystemStorage(storagePath)
	if err != nil {
		logger.Fatal("Failed to initialize storage:", err)
	}

	server := &Server{
		storage: storage,
		logger:  logger,
		token:   authToken,
	}

	// Setup routes
	http.HandleFunc("/v8/artifacts/events", server.handleAuth(server.recordEvents))
	http.HandleFunc("/v8/artifacts/status", server.handleAuth(server.getStatus))
	http.HandleFunc("/v8/artifacts/", server.handleAuth(server.handleArtifact))
	http.HandleFunc("/v8/artifacts", server.handleAuth(server.queryArtifacts))

	server.logger.Printf("Starting server on :8080")
	fmt.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		server.logger.Fatal(err)
	}
}

// Middleware to handle authentication
func (s *Server) handleAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := newLoggingResponseWriter(w)

		// Log request
		s.logger.Printf("Request: %s %s", r.Method, r.URL.Path)

		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(lrw, "Unauthorized", http.StatusUnauthorized)
			s.logger.Printf("Response: %d Unauthorized (no bearer token) - %v",
				http.StatusUnauthorized, time.Since(start))
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		if token != s.token {
			http.Error(lrw, "Unauthorized", http.StatusUnauthorized)
			s.logger.Printf("Response: %d Unauthorized (invalid token) - %v",
				http.StatusUnauthorized, time.Since(start))
			return
		}

		next(lrw, r)

		// Log response
		s.logger.Printf("Response: %d %s - %v",
			lrw.statusCode, http.StatusText(lrw.statusCode), time.Since(start))
	}
}

// Handler for /v8/artifacts/events
func (s *Server) recordEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var events []ArtifactEvent
	if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Log events
	for _, event := range events {
		s.logger.Printf("Cache event: %s %s %s (duration: %.2f)",
			event.Hash, event.Source, event.Event, event.Duration)
	}

	w.WriteHeader(http.StatusOK)
}

// Handler for /v8/artifacts/status
func (s *Server) getStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := StatusResponse{
		Status: "enabled",
	}

	json.NewEncoder(w).Encode(response)
}

// Handler for /v8/artifacts/{hash}
func (s *Server) handleArtifact(w http.ResponseWriter, r *http.Request) {
	hash := strings.TrimPrefix(r.URL.Path, "/v8/artifacts/")

	switch r.Method {
	case http.MethodGet:
		s.downloadArtifact(w, r, hash)
	case http.MethodPut:
		s.uploadArtifact(w, r, hash)
	case http.MethodHead:
		s.checkArtifact(w, r, hash)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) downloadArtifact(w http.ResponseWriter, r *http.Request, hash string) {
	reader, size, err := s.storage.Get(hash)
	if err != nil {
		s.logger.Printf("Download failed for hash %s: %v", hash, err)
		http.Error(w, "Artifact not found", http.StatusNotFound)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
	w.Header().Set("Content-Type", "application/octet-stream")

	if _, err := io.Copy(w, reader); err != nil {
		s.logger.Printf("Error streaming artifact %s: %v", hash, err)
		return
	}
}

func (s *Server) uploadArtifact(w http.ResponseWriter, r *http.Request, hash string) {
	contentLength := r.Header.Get("Content-Length")
	if contentLength == "" {
		http.Error(w, "Content-Length required", http.StatusBadRequest)
		return
	}

	if err := s.storage.Store(hash, r.Body); err != nil {
		s.logger.Printf("Upload failed for hash %s: %v", hash, err)
		http.Error(w, "Failed to store artifact", http.StatusInternalServerError)
		return
	}

	response := UploadResponse{
		URLs: []string{
			fmt.Sprintf("https://api.vercel.com/v2/now/artifact/%s", hash),
		},
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(response)
}

func (s *Server) checkArtifact(w http.ResponseWriter, r *http.Request, hash string) {
	exists, err := s.storage.Exists(hash)
	if err != nil {
		s.logger.Printf("Error checking artifact %s: %v", hash, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !exists {
		http.Error(w, "Artifact not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Handler for /v8/artifacts (POST - query)
func (s *Server) queryArtifacts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ArtifactQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	response := make(map[string]*ArtifactInfo)
	for _, hash := range req.Hashes {
		reader, size, err := s.storage.Get(hash)
		if err != nil {
			response[hash] = &ArtifactInfo{
				Error: &struct {
					Message string `json:"message"`
				}{
					Message: "Artifact not found",
				},
			}
			continue
		}
		reader.Close()

		response[hash] = &ArtifactInfo{
			Size: int(size),
		}
	}

	json.NewEncoder(w).Encode(response)
}

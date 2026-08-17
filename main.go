package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"fprinter/config"
	"fprinter/escpos"
	"fprinter/models"
	"fprinter/printer"
)

const serviceName = "FprinterAgent"

type response struct {
	Ok      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Printer string `json:"printer,omitempty"`
}

// server encapsula la app HTTP. Lo aislamos para poder start/shutdown
// de forma controlada desde el handler del servicio Windows.
type server struct {
	cfg *config.Config
	srv *http.Server
}

func newServer(cfg *config.Config) *server {
	s := &server{cfg: cfg}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/print", s.withCORS(s.handlePrint))

	s.srv = &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", cfg.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	return s
}

func (s *server) start() error {
	log.Printf("FprinterAgent escuchando en %s, receipt: %s", s.srv.Addr, s.cfg.Printers.Receipt.PrinterName)
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *server) shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

// ─── CORS ───────────────────────────────────────────────────────────────

func (s *server) withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if s.isOriginAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

func (s *server) isOriginAllowed(origin string) bool {
	for _, a := range s.cfg.AllowedOrigins {
		if a == origin {
			return true
		}
	}
	return false
}

// ─── Handlers ───────────────────────────────────────────────────────────

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, response{Ok: true, Printer: s.cfg.Printers.Receipt.PrinterName})
}

func (s *server) handlePrint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response{Ok: false, Error: "método no permitido"})
		return
	}

	if r.Header.Get("Authorization") != "Bearer "+s.cfg.Token {
		log.Printf("Token inválido desde %s", r.RemoteAddr)
		writeJSON(w, http.StatusUnauthorized, response{Ok: false, Error: "unauthorized"})
		return
	}

	var req models.PrintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Ok: false, Error: "JSON inválido: " + err.Error()})
		return
	}

	rc := s.cfg.Printers.Receipt
	bytes, err := escpos.Render(&req, rc.PaperWidth, rc.CodePage)
	if err != nil {
		log.Printf("Validación: %v", err)
		writeJSON(w, http.StatusBadRequest, response{Ok: false, Error: err.Error()})
		return
	}

	if err := printer.SendRaw(rc.PrinterName, bytes); err != nil {
		log.Printf("Error imprimiendo: %v", err)
		writeJSON(w, http.StatusInternalServerError, response{Ok: false, Error: err.Error()})
		return
	}

	log.Printf("Ticket impreso (%d líneas, %d bytes)", len(req.Lines), len(bytes))
	writeJSON(w, http.StatusOK, response{Ok: true})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// ─── Entry point ────────────────────────────────────────────────────────

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Error cargando config: %v", err)
	}

	// run() está en service_windows.go o service_other.go según build tag.
	// En Windows detecta automáticamente si corre como servicio o consola.
	if err := run(cfg); err != nil {
		log.Fatalf("Error fatal: %v", err)
	}
}

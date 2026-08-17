//go:build windows

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"fprinter/config"

	"golang.org/x/sys/windows/svc"
)

// run detecta automáticamente si el proceso corre como servicio Windows
// (lanzado por el SCM) o interactivo (consola), y arranca el modo correspondiente.
func run(cfg *config.Config) error {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return fmt.Errorf("svc.IsWindowsService: %w", err)
	}

	if isService {
		setupLogFile()
		return svc.Run(serviceName, &winService{cfg: cfg})
	}
	if cfg.Debug {
		allocConsole()
		return newServer(cfg).start()
	}
	return runTray(cfg)
}

func logPath() string {
	exePath, _ := os.Executable()
	return filepath.Join(filepath.Dir(exePath), "fprinter.log")
}

func setupLogFile() {
	p := logPath()
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("No se pudo abrir log %s: %v", p, err)
		return
	}
	log.SetOutput(f)
	log.SetFlags(log.Ldate | log.Ltime)
}

// ─── Handler del servicio Windows ──────────────────────────────────────

type winService struct {
	cfg *config.Config
}

// Execute es el callback del SCM. Recibe comandos (start/stop/shutdown) por r
// y debe reportar estado por status. El SCM matará el servicio si no responde
// a tiempo (timeout de start ~30s), así que el patrón es:
//  1. reportar StartPending
//  2. arrancar trabajo en goroutine
//  3. reportar Running
//  4. loop esperando comandos
//  5. al stop: reportar StopPending → cleanup → Stopped
func (w *winService) Execute(args []string, r <-chan svc.ChangeRequest, status chan<- svc.Status) (ssec bool, errno uint32) {
	const accepts = svc.AcceptStop | svc.AcceptShutdown

	status <- svc.Status{State: svc.StartPending}

	srv := newServer(w.cfg)

	// HTTP server en goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.start()
	}()

	// Pequeña espera para detectar fallos inmediatos (puerto ocupado, etc.)
	time.Sleep(200 * time.Millisecond)

	status <- svc.Status{State: svc.Running, Accepts: accepts}
	log.Print("Servicio en estado Running")

loop:
	for {
		select {
		case err := <-errCh:
			log.Printf("HTTP server terminó inesperadamente: %v", err)
			break loop

		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				log.Print("Recibido stop/shutdown")
				break loop
			default:
				log.Printf("Comando inesperado: %v", c.Cmd)
			}
		}
	}

	status <- svc.Status{State: svc.StopPending}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.shutdown(ctx); err != nil {
		log.Printf("Shutdown: %v", err)
	}

	status <- svc.Status{State: svc.Stopped}
	return false, 0
}

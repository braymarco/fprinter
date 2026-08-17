//go:build windows

package main

import (
	"context"
	"encoding/binary"
	"log"
	"os"
	"os/exec"
	"time"

	"fprinter/config"

	"github.com/getlantern/systray"
	"golang.org/x/sys/windows"
)

func runTray(cfg *config.Config) error {
	setupLogFile()

	log.Printf("FprinterAgent iniciando — puerto: %d, impresora: %s", cfg.Port, cfg.Printers.Receipt.PrinterName)

	srv := newServer(cfg)
	errCh := make(chan error, 1)
	go func() { errCh <- srv.start() }()

	// Canal para propagar el error del servidor tras el cierre del tray
	srvErrCh := make(chan error, 1)

	systray.Run(
		func() {
			systray.SetIcon(makeICO())
			systray.SetTooltip("FprinterAgent")

			mInfo := systray.AddMenuItem("FprinterAgent", "")
			mInfo.Disable()
			systray.AddSeparator()
			mLog := systray.AddMenuItem("Ver log", "")
			mExit := systray.AddMenuItem("Salir", "")

			lp := logPath()
			go func() {
				for {
					select {
					case <-mLog.ClickedCh:
						exec.Command("cmd", "/c", "start", "", lp).Start()
					case <-mExit.ClickedCh:
						srvErrCh <- nil
						systray.Quit()
						return
					case err := <-errCh:
						log.Printf("Servidor terminó inesperadamente: %v", err)
						srvErrCh <- err
						systray.Quit()
						return
					}
				}
			}()
		},
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := srv.shutdown(ctx); err != nil {
				log.Printf("Shutdown: %v", err)
			}
		},
	)

	select {
	case err := <-srvErrCh:
		return err
	default:
		return nil
	}
}

// allocConsole abre una consola cuando el exe se compiló con -H windowsgui.
// Úsalo solo en modo debug.
func allocConsole() {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	kernel32.NewProc("AllocConsole").Call()
	conout, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err != nil {
		return
	}
	os.Stdout = conout
	os.Stderr = conout
	log.SetOutput(conout)
}

// makeICO genera un ICO 16×16 de 32 bpp con un cuadrado sólido azul-verde.
// Reemplazar con un ICO real para mejor apariencia.
func makeICO() []byte {
	const (
		w = 16
		h = 16
	)

	// Datos de píxeles BGRA, orden bottom-to-top
	xor := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		xor[i*4+0] = 0x6A // B
		xor[i*4+1] = 0xA0 // G
		xor[i*4+2] = 0x20 // R
		xor[i*4+3] = 0xFF // A
	}

	// Máscara AND: ceil(w/32)*4 bytes por fila, todo cero = totalmente opaco
	andStride := ((w + 31) / 32) * 4
	and := make([]byte, andStride*h)

	// BITMAPINFOHEADER (40 bytes)
	bih := make([]byte, 40)
	binary.LittleEndian.PutUint32(bih[0:], 40)
	binary.LittleEndian.PutUint32(bih[4:], uint32(w))
	binary.LittleEndian.PutUint32(bih[8:], uint32(h*2)) // altura doble: XOR + AND
	binary.LittleEndian.PutUint16(bih[12:], 1)
	binary.LittleEndian.PutUint16(bih[14:], 32)

	imgData := append(bih, xor...)
	imgData = append(imgData, and...)

	var sizeBuf [4]byte
	binary.LittleEndian.PutUint32(sizeBuf[:], uint32(len(imgData)))
	var offBuf [4]byte
	binary.LittleEndian.PutUint32(offBuf[:], 22) // 6 (ICONDIR) + 16 (ICONDIRENTRY)

	ico := []byte{
		0x00, 0x00, // ICONDIR: reserved
		0x01, 0x00, // type = ICO
		0x01, 0x00, // count = 1
		// ICONDIRENTRY:
		byte(w), byte(h), // width, height
		0x00, 0x00, // color count, reserved
		0x01, 0x00, // planes
		0x20, 0x00, // bit count = 32
	}
	ico = append(ico, sizeBuf[:]...)
	ico = append(ico, offBuf[:]...)
	ico = append(ico, imgData...)
	return ico
}

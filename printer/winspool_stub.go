//go:build !windows

package printer

import "fmt"

// SendRaw stub para plataformas no-Windows.
// Útil en desarrollo: permite testear toda la lógica desde Linux/Mac
// imprimiendo los bytes ESC/POS a stdout en hex.
func SendRaw(printerName string, data []byte) error {
	fmt.Printf("[DEV] Imprimiría %d bytes en %q\n", len(data), printerName)
	fmt.Printf("[DEV] Hex: %x\n", data)
	return nil
}

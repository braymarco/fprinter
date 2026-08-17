package escpos

import (
	"fmt"
	"regexp"
	"strings"

	"fprinter/models"
)

// Render traduce un PrintRequest a bytes ESC/POS listos para enviar.
// paperWidth es el ancho del papel en columnas (32 para 58mm, 48 para 80mm)
// y se usa por defecto en separadores horizontales.
// Valida formato de barcodes antes de generar para fallar rápido y claro.
func Render(req *models.PrintRequest, paperWidth int, codePage string) ([]byte, error) {
	if req == nil || len(req.Lines) == 0 {
		return nil, fmt.Errorf("el ticket no tiene líneas")
	}

	b := NewBuilder(codePage)

	for i := range req.Lines {
		if err := renderLine(b, &req.Lines[i], paperWidth); err != nil {
			return nil, fmt.Errorf("línea %d: %w", i, err)
		}
	}

	return b.Bytes(), nil
}

func renderLine(b *Builder, line *models.TicketLine, paperWidth int) error {
	switch strings.ToLower(line.Type) {
	case "text":
		return renderText(b, line, paperWidth)

	case "qr":
		if line.Content == "" {
			return fmt.Errorf("qr requiere 'content'")
		}
		size := line.QrSize
		if size == 0 {
			size = 6
		}
		ec := line.QrErrorCorrection
		if ec == "" {
			ec = "M"
		}
		b.Align(line.Align).QR(line.Content, size, ec).Align("left")
		return nil

	case "barcode":
		if line.Content == "" {
			return fmt.Errorf("barcode requiere 'content'")
		}
		bt := line.BarcodeType
		if bt == "" {
			bt = "CODE128"
		}
		if err := validateBarcode(bt, line.Content); err != nil {
			return err
		}
		height, width := line.BarcodeHeight, line.BarcodeWidth
		if height == 0 {
			height = 80
		}
		if width == 0 {
			width = 3
		}
		hri := line.BarcodeHri
		if hri == "" {
			hri = "below"
		}
		b.Align(line.Align)
		if err := b.Barcode(line.Content, bt, height, width, hri); err != nil {
			return err
		}
		b.Align("left").LineBreak()
		return nil

	case "feed":
		n := line.Lines
		if n == 0 {
			n = 1
		}
		b.Feed(n)
		return nil

	case "hr":
		char := line.Char
		if char == "" {
			char = "-"
		}
		// Solo permitimos un carácter ASCII como separador
		// (multibyte rompe el conteo de columnas en CP850)
		runes := []rune(char)
		if len(runes) != 1 || runes[0] > 0x7E {
			return fmt.Errorf("hr 'char' debe ser un carácter ASCII (recibido: %q)", char)
		}
		width := line.Width
		if width == 0 || width > paperWidth {
			width = paperWidth
		}
		if width < 1 {
			return fmt.Errorf("hr 'width' fuera de rango: %d", width)
		}
		b.Align("left").ResetStyle().Text(strings.Repeat(char, width))
		return nil

	case "cut":
		b.Cut()
		return nil

	case "cashdraw":
		b.CashDrawer(2)
		return nil

	default:
		return fmt.Errorf("tipo de línea desconocido: %q", line.Type)
	}
}

func renderText(b *Builder, line *models.TicketLine, paperWidth int) error {
	b.Align(line.Align)

	if len(line.Segments) > 0 {
		for _, seg := range line.Segments {
			size := seg.Size
			if size == 0 {
				size = 1
			}
			text := seg.Text
			if seg.Width > 0 {
				cols := paperWidth * seg.Width / 100
				text = fitToWidth(text, cols, seg.Align)
			}
			b.Bold(seg.Bold).Size(size, size).TextRaw(text)
		}
		b.LineBreak()
	} else {
		size := line.Size
		if size == 0 {
			size = 1
		}
		b.Bold(line.Bold).Size(size, size).Text(line.Content)
	}

	b.ResetStyle()
	return nil
}

func fitToWidth(s string, cols int, align string) string {
	runes := []rune(s)
	if len(runes) >= cols {
		return string(runes[:cols])
	}
	padding := cols - len(runes)
	switch align {
	case "right":
		return strings.Repeat(" ", padding) + s
	case "center":
		left := padding / 2
		right := padding - left
		return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
	default: // left
		return s + strings.Repeat(" ", padding)
	}
}

// ─── Validaciones de barcode ────────────────────────────────────────────

var (
	digitsOnly  = regexp.MustCompile(`^[0-9]+$`)
	code39Chars = regexp.MustCompile(`^[0-9A-Z\-\. \$/\+%]+$`)
)

func validateBarcode(t, data string) error {
	switch strings.ToUpper(t) {
	case "EAN13", "JAN13":
		if !digitsOnly.MatchString(data) || (len(data) != 12 && len(data) != 13) {
			return fmt.Errorf("EAN13 requiere 12 o 13 dígitos")
		}
	case "EAN8", "JAN8":
		if !digitsOnly.MatchString(data) || (len(data) != 7 && len(data) != 8) {
			return fmt.Errorf("EAN8 requiere 7 u 8 dígitos")
		}
	case "UPCA":
		if !digitsOnly.MatchString(data) || (len(data) != 11 && len(data) != 12) {
			return fmt.Errorf("UPCA requiere 11 o 12 dígitos")
		}
	case "UPCE":
		if !digitsOnly.MatchString(data) || len(data) < 6 || len(data) > 8 {
			return fmt.Errorf("UPCE requiere 6-8 dígitos")
		}
	case "ITF":
		if !digitsOnly.MatchString(data) || len(data)%2 != 0 || len(data) < 2 {
			return fmt.Errorf("ITF requiere cantidad par de dígitos")
		}
	case "CODE39":
		if !code39Chars.MatchString(data) {
			return fmt.Errorf("CODE39 solo acepta 0-9 A-Z - . $ / + %% espacio")
		}
	case "CODE128":
		for _, c := range data {
			if c < 0x20 || c > 0x7E {
				return fmt.Errorf("CODE128 solo acepta ASCII imprimible")
			}
		}
	}
	return nil
}

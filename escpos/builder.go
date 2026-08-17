package escpos

import (
	"fmt"
	"strings"

	"golang.org/x/text/encoding/charmap"
)

// Builder acumula bytes ESC/POS. Recordar que ESC/POS es una máquina de estados:
// bold, alineación y tamaño son MODALES, persisten hasta cambiarse explícitamente.
// El renderer es responsable de resetear estilos al terminar cada línea.
type Builder struct {
	buf []byte
	cm  *charmap.Charmap
}

// Comandos ESC/POS básicos
var (
	cmdInit        = []byte{0x1B, 0x40}
	cmdCutFull     = []byte{0x1D, 0x56, 0x00}
	cmdAlignLeft   = []byte{0x1B, 0x61, 0x00}
	cmdAlignCenter = []byte{0x1B, 0x61, 0x01}
	cmdAlignRight  = []byte{0x1B, 0x61, 0x02}
	cmdBoldOn      = []byte{0x1B, 0x45, 0x01}
	cmdBoldOff     = []byte{0x1B, 0x45, 0x00}
)

// NewBuilder crea un builder con el code page indicado.
// Valores soportados: "CP850" (default), "CP1252", "CP437".
func NewBuilder(codePage string) *Builder {
	b := &Builder{}

	var escT byte
	switch strings.ToUpper(codePage) {
	case "CP1252", "WINDOWS1252":
		b.cm = charmap.Windows1252
		escT = 0x10
	case "CP437":
		b.cm = charmap.CodePage437
		escT = 0x00
	default: // CP850
		b.cm = charmap.CodePage850
		escT = 0x02
	}

	b.buf = append(b.buf, cmdInit...)
	b.buf = append(b.buf, 0x1B, 0x74, escT)
	return b
}

func (b *Builder) Bytes() []byte { return b.buf }

// ─── Estilos ────────────────────────────────────────────────────────────

func (b *Builder) Align(a string) *Builder {
	switch a {
	case "center":
		b.buf = append(b.buf, cmdAlignCenter...)
	case "right":
		b.buf = append(b.buf, cmdAlignRight...)
	default:
		b.buf = append(b.buf, cmdAlignLeft...)
	}
	return b
}

func (b *Builder) Size(width, height int) *Builder {
	w := clamp(width, 1, 8) - 1
	h := clamp(height, 1, 8) - 1
	n := byte((w << 4) | h)
	b.buf = append(b.buf, 0x1D, 0x21, n)
	return b
}

func (b *Builder) Bold(on bool) *Builder {
	if on {
		b.buf = append(b.buf, cmdBoldOn...)
	} else {
		b.buf = append(b.buf, cmdBoldOff...)
	}
	return b
}

func (b *Builder) ResetStyle() *Builder {
	b.Bold(false)
	b.Size(1, 1)
	return b
}

// ─── Texto ──────────────────────────────────────────────────────────────

var accentReplacer = strings.NewReplacer(
	"á", "a", "Á", "A",
	"é", "e", "É", "E",
	"í", "i", "Í", "I",
	"ó", "o", "Ó", "O",
	"ú", "u", "Ú", "U",
	"ü", "u", "Ü", "U",
	"ñ", "n", "Ñ", "N",
)

// TextRaw escribe sin salto de línea (para componer líneas con segmentos).
func (b *Builder) TextRaw(s string) *Builder {
	s = accentReplacer.Replace(s)
	// UTF-8 → code page configurado. Caracteres no representables → '?'
	mapped := strings.Map(func(r rune) rune {
		if _, ok := b.cm.EncodeRune(r); ok {
			return r
		}
		return '?'
	}, s)
	encoded, err := b.cm.NewEncoder().String(mapped)
	if err != nil {
		encoded = mapped
	}
	b.buf = append(b.buf, []byte(encoded)...)
	return b
}

func (b *Builder) Text(s string) *Builder {
	return b.TextRaw(s).LineBreak()
}

func (b *Builder) LineBreak() *Builder {
	b.buf = append(b.buf, 0x0A)
	return b
}

func (b *Builder) Feed(n int) *Builder {
	n = clamp(n, 1, 50)
	for i := 0; i < n; i++ {
		b.buf = append(b.buf, 0x0A)
	}
	return b
}

// ─── Corte y cajón ──────────────────────────────────────────────────────

func (b *Builder) Cut() *Builder {
	b.buf = append(b.buf, cmdCutFull...)
	return b
}

func (b *Builder) CashDrawer(pin int) *Builder {
	m := byte(0)
	if pin == 5 {
		m = 1
	}
	b.buf = append(b.buf, 0x1B, 0x70, m, 0x19, 0xFA)
	return b
}

// ─── QR ─────────────────────────────────────────────────────────────────

// QR imprime un código QR modelo 2. size: 1-16. ec: "L", "M", "Q", "H".
func (b *Builder) QR(data string, size int, ec string) *Builder {
	sz := byte(clamp(size, 1, 16))
	var ecCode byte
	switch strings.ToUpper(ec) {
	case "L":
		ecCode = 0x30
	case "Q":
		ecCode = 0x32
	case "H":
		ecCode = 0x33
	default:
		ecCode = 0x31 // M
	}

	dataBytes := []byte(data)
	length := len(dataBytes) + 3
	pL := byte(length & 0xFF)
	pH := byte((length >> 8) & 0xFF)

	// Función 165: modelo (2)
	b.buf = append(b.buf, 0x1D, 0x28, 0x6B, 0x04, 0x00, 0x31, 0x41, 0x32, 0x00)
	// Función 167: tamaño módulo
	b.buf = append(b.buf, 0x1D, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x43, sz)
	// Función 169: corrección error
	b.buf = append(b.buf, 0x1D, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x45, ecCode)
	// Función 180: almacenar datos
	b.buf = append(b.buf, 0x1D, 0x28, 0x6B, pL, pH, 0x31, 0x50, 0x30)
	b.buf = append(b.buf, dataBytes...)
	// Función 181: imprimir
	b.buf = append(b.buf, 0x1D, 0x28, 0x6B, 0x03, 0x00, 0x31, 0x51, 0x30)

	return b
}

// ─── Barcode ────────────────────────────────────────────────────────────

var barcodeTypes = map[string]byte{
	"UPCA": 65, "UPCE": 66,
	"EAN13": 67, "JAN13": 67,
	"EAN8": 68, "JAN8": 68,
	"CODE39":  69,
	"ITF":     70,
	"CODABAR": 71,
	"CODE93":  72,
	"CODE128": 73,
}

// Barcode imprime un código de barras 1D vía función B de GS k.
func (b *Builder) Barcode(data, btype string, height, width int, hri string) error {
	typeCode, ok := barcodeTypes[strings.ToUpper(btype)]
	if !ok {
		return fmt.Errorf("tipo de barcode no soportado: %s", btype)
	}

	b.buf = append(b.buf, 0x1D, 0x68, byte(clamp(height, 1, 255)))  // GS h n
	b.buf = append(b.buf, 0x1D, 0x77, byte(clamp(width, 2, 6)))     // GS w n

	var hriPos byte
	switch strings.ToLower(hri) {
	case "above":
		hriPos = 1
	case "below":
		hriPos = 2
	case "both":
		hriPos = 3
	}
	b.buf = append(b.buf, 0x1D, 0x48, hriPos) // GS H n

	dataBytes := []byte(data)
	// CODE128 requiere prefijo de set
	if strings.EqualFold(btype, "CODE128") && !strings.HasPrefix(data, "{") {
		prefixed := make([]byte, 0, len(dataBytes)+2)
		prefixed = append(prefixed, '{', 'B')
		prefixed = append(prefixed, dataBytes...)
		dataBytes = prefixed
	}

	if len(dataBytes) > 255 {
		return fmt.Errorf("barcode demasiado largo (%d bytes, máx 255)", len(dataBytes))
	}

	// GS k m n d1...dn (función B)
	b.buf = append(b.buf, 0x1D, 0x6B, typeCode, byte(len(dataBytes)))
	b.buf = append(b.buf, dataBytes...)
	return nil
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

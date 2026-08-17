package models

// PrintRequest es el cuerpo del POST /print.
type PrintRequest struct {
	Lines []TicketLine `json:"lines"`
}

// TicketLine representa una operación del ticket.
// El campo Type determina qué otros campos se usan:
//   - "text"     → Content o Segments + Align/Bold/Size
//   - "qr"       → Content + QrSize + QrErrorCorrection
//   - "barcode"  → Content + BarcodeType + BarcodeHeight + BarcodeWidth + BarcodeHri
//   - "hr"       → Char (default "-") + Width (default = PaperWidth de config)
//   - "feed"     → Lines
//   - "cut"      → (sin params)
//   - "cashdraw" → (sin params)
type TicketLine struct {
	Type     string        `json:"type"`
	Content  string        `json:"content,omitempty"`
	Segments []TextSegment `json:"segments,omitempty"`

	// Texto
	Align string `json:"align,omitempty"` // left | center | right
	Bold  bool   `json:"bold,omitempty"`
	Size  int    `json:"size,omitempty"` // 1-8

	// Feed
	Lines int `json:"lines,omitempty"`

	// QR
	QrSize            int    `json:"qrSize,omitempty"`            // 1-16
	QrErrorCorrection string `json:"qrErrorCorrection,omitempty"` // L | M | Q | H

	// Barcode
	BarcodeType   string `json:"barcodeType,omitempty"`
	BarcodeHeight int    `json:"barcodeHeight,omitempty"` // 1-255
	BarcodeWidth  int    `json:"barcodeWidth,omitempty"`  // 2-6
	BarcodeHri    string `json:"barcodeHri,omitempty"`    // none | above | below | both

	// HR (separador horizontal)
	Char  string `json:"char,omitempty"`  // carácter a repetir, default "-"
	Width int    `json:"width,omitempty"` // ancho en columnas, default = PaperWidth
}

// TextSegment es un fragmento con estilo propio dentro de una línea de texto.
type TextSegment struct {
	Text  string `json:"text"`
	Bold  bool   `json:"bold,omitempty"`
	Size  int    `json:"size,omitempty"`
	Width int    `json:"width,omitempty"` // porcentaje del ancho del papel (0 = sin restricción)
	Align string `json:"align,omitempty"` // left (default) | center | right
}

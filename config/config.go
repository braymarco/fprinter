package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type LabelOffset struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type ReceiptConfig struct {
	Enabled     bool   `json:"enabled"`
	PrinterName string `json:"printerName"`
	PaperWidth  int    `json:"paperWidth"` // mm: 58 → 32 cols, 80 → 48 cols
	Driver      string `json:"driver"`     // "escpos"
	CodePage    string `json:"codePage"`   // "CP850" (default), "CP1252", "CP437"
}

type LabelConfig struct {
	Enabled     bool        `json:"enabled"`
	PrinterName string      `json:"printerName"`
	Driver      string      `json:"driver"` // "tspl"
	DPI         int         `json:"dpi"`
	LabelWidth  int         `json:"labelWidth"`  // mm
	LabelHeight int         `json:"labelHeight"` // mm
	Gap         int         `json:"gap"`         // mm
	Sensor      string      `json:"sensor"`      // "gap" | "bline"
	Density     int         `json:"density"`     // 0-15
	Encoding    string      `json:"encoding"`
	Offset      LabelOffset `json:"offset"`
}

type PrintersConfig struct {
	Receipt ReceiptConfig `json:"receipt"`
	Label   LabelConfig   `json:"label"`
}

type Config struct {
	Port           int            `json:"port"`
	Token          string         `json:"token"`
	AllowedOrigins []string       `json:"allowedOrigins"`
	Debug          bool           `json:"debug"`
	Printers       PrintersConfig `json:"printers"`
}

// Load lee config.json desde el directorio del ejecutable (NO el cwd).
// Esto es importante porque al correr como servicio Windows el cwd es System32.
func Load() (*Config, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("obteniendo path del ejecutable: %w", err)
	}
	cfgPath := filepath.Join(filepath.Dir(exePath), "config.json")

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("leyendo %s: %w", cfgPath, err)
	}

	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parseando %s: %w", cfgPath, err)
	}

	if c.Port == 0 {
		c.Port = 8765
	}
	if c.Token == "" {
		return nil, fmt.Errorf("falta 'token' en config.json")
	}

	// ── Receipt defaults ──────────────────────────────────────────────────
	r := &c.Printers.Receipt
	if r.Enabled {
		if r.PrinterName == "" {
			return nil, fmt.Errorf("falta 'printers.receipt.printerName' en config.json")
		}
		if r.PaperWidth == 0 {
			r.PaperWidth = 80
		}
		if cols, ok := map[int]int{58: 32, 80: 48}[r.PaperWidth]; ok {
			r.PaperWidth = cols
		}
		if r.CodePage == "" {
			r.CodePage = "CP850"
		}
		if r.Driver == "" {
			r.Driver = "escpos"
		}
	}

	// ── Label defaults ────────────────────────────────────────────────────
	l := &c.Printers.Label
	if l.Enabled {
		if l.PrinterName == "" {
			return nil, fmt.Errorf("falta 'printers.label.printerName' en config.json")
		}
		if l.DPI == 0 {
			l.DPI = 203
		}
		if l.Encoding == "" {
			l.Encoding = "cp850"
		}
		if l.Driver == "" {
			l.Driver = "tspl"
		}
	}

	return &c, nil
}

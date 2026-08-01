# FprinterAgent (Go)

Agente local Windows para impresión térmica ESC/POS sin diálogos del navegador.
Versión en Go: binario único de ~8 MB, cross-compilable desde cualquier OS, sin runtime.

## Arquitectura

```
[Navegador JS] --HTTP localhost:8765--> [FprinterAgent.exe] --RAW--> [Spooler] --USB--> [Impresora]
```

- Binario único nativo, sin runtime, sin CGO, sin dependencias externas.
- Detecta automáticamente si corre como servicio Windows o consola (modo desarrollo).
- Bind solo a `127.0.0.1` (no expuesto a la red).
- Auth por bearer token + CORS estricto por origen.
- Stub multiplataforma: podés desarrollar en Linux/Mac y los bytes se imprimen a stdout.

## Estructura

```
print-agent-go/
├── go.mod
├── main.go                       # HTTP server, handlers, entry point
├── service_windows.go            # Service handler para SCM (build tag windows)
├── service_other.go              # Stub no-Windows (modo consola)
├── config/config.go              # Lee config.json desde dir del exe
├── models/ticket.go              # Tipos del request
├── escpos/
│   ├── builder.go                # Comandos ESC/POS de bajo nivel
│   └── renderer.go               # Modelo → bytes + validaciones
├── printer/
│   ├── winspool_windows.go       # winspool.drv vía syscall
│   └── winspool_stub.go          # Stub para dev en Linux/Mac
├── config.json
├── build.sh                      # Cross-compile a Windows
├── install-service.ps1           # Instalador del servicio
└── README.md
```

## Compilar

Requisitos: Go 1.22+.

Desde cualquier OS con bash:

```bash
./build.sh
```

Desde Windows sin bash:

```powershell
go build -ldflags -H=windowsgui
$env:GOOS="windows"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"
go build -ldflags="-s -w" -trimpath -o dist\FprinterAgent.exe .
Copy-Item config.json dist\
Copy-Item install-service.ps1 dist\
```

Resultado: `dist/FprinterAgent.exe` (~8 MB) + `config.json` + `install-service.ps1`.

## Desplegar en PC destino

### 1. Preparar la impresora

Igual que cualquier solución ESC/POS:
1. Conectar la Xprinter/3nstar por USB.
2. Panel de control → Dispositivos e impresoras → Agregar impresora local manual.
3. Driver: **Generic / Text Only** (viene con Windows).
4. Anotar el nombre exacto.

### 2. Instalar el agente

Copiar `dist/` a la PC destino. Editar `config.json`:

```json
{
  "port": 8765,
  "token": "uuid-único-de-esta-máquina",
  "printerName": "XP-80C",
  "allowedOrigins": ["https://tu-app.com"]
}
```

PowerShell como Administrador:

```powershell
.\install-service.ps1 install
```

### 3. Verificar

```powershell
curl http://127.0.0.1:8765/health
# {"ok":true,"printer":"XP-80C"}
```

## Comandos del servicio

```powershell
.\install-service.ps1 status
.\install-service.ps1 restart      # Necesario tras editar config.json
.\install-service.ps1 uninstall
```

## Desarrollo local (Linux/Mac)

```bash
go run .
```

En otra terminal:

```bash
curl -X POST http://127.0.0.1:8765/print \
  -H "Authorization: Bearer CAMBIA-ESTO-POR-UN-UUID-LARGO-Y-UNICO" \
  -H "Content-Type: application/json" \
  -d '{
    "lines": [
      {"type": "text", "content": "HOLA MUNDO", "bold": true, "align": "center", "size": 2},
      {"type": "qr", "content": "https://example.com", "align": "center"},
      {"type": "cut"}
    ]
  }'
```

El stub imprime los bytes en hex en la consola del agente, útil para validar
que el modelo se traduce bien antes de probar en hardware real.

## Uso desde el cliente JS

```javascript
await fetch('http://127.0.0.1:8765/print', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${TOKEN}`,
  },
  body: JSON.stringify({
    lines: [
      { type: 'text', content: 'MI TIENDA', align: 'center', bold: true, size: 2 },
      { type: 'hr' },
      {
        type: 'text',
        align: 'left',
        segments: [
          { text: 'TOTAL: ', bold: true, size: 2 },
          { text: '$25.00' },
        ],
      },
      { type: 'hr', char: '=' },
      { type: 'barcode', content: '7501234567890', barcodeType: 'EAN13', align: 'center' },
      { type: 'qr', content: 'https://miapp.com/t/123', align: 'center', qrSize: 6 },
      { type: 'feed', lines: 2 },
      { type: 'cut' },
    ],
  }),
});
```

## Modelo del ticket

| `type`     | Campos                                                                |
|------------|-----------------------------------------------------------------------|
| `text`     | `content` o `segments`, `align`, `bold`, `size` (1-8)                 |
| `qr`       | `content`, `align`, `qrSize` (1-16), `qrErrorCorrection` (L/M/Q/H)    |
| `barcode`  | `content`, `barcodeType`, `barcodeHeight`, `barcodeWidth`, `barcodeHri`, `align` |
| `hr`       | `char` (default `-`), `width` (default = `paperWidth` de config)      |
| `feed`     | `lines` (1-50)                                                        |
| `cut`      | -                                                                     |
| `cashdraw` | -                                                                     |

Tipos de barcode: `EAN13`, `EAN8`, `UPCA`, `UPCE`, `CODE39`, `CODE128`, `ITF`, `CODABAR`, `CODE93`.
"# fprinter" 

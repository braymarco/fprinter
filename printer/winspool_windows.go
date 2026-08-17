//go:build windows

package printer

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	winspool = windows.NewLazySystemDLL("winspool.drv")

	procOpenPrinterW     = winspool.NewProc("OpenPrinterW")
	procClosePrinter     = winspool.NewProc("ClosePrinter")
	procStartDocPrinterW = winspool.NewProc("StartDocPrinterW")
	procEndDocPrinter    = winspool.NewProc("EndDocPrinter")
	procStartPagePrinter = winspool.NewProc("StartPagePrinter")
	procEndPagePrinter   = winspool.NewProc("EndPagePrinter")
	procWritePrinter     = winspool.NewProc("WritePrinter")
)

// docInfo1 corresponde a DOC_INFO_1W de winspool.h
type docInfo1 struct {
	DocName    *uint16
	OutputFile *uint16
	Datatype   *uint16
}

// SendRaw envía bytes RAW al spooler de Windows.
// El driver debe respetar RAW (Generic / Text Only lo hace).
func SendRaw(printerName string, data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("data vacía")
	}

	pName, err := windows.UTF16PtrFromString(printerName)
	if err != nil {
		return fmt.Errorf("nombre de impresora inválido: %w", err)
	}

	var hPrinter windows.Handle
	r1, _, e1 := procOpenPrinterW.Call(
		uintptr(unsafe.Pointer(pName)),
		uintptr(unsafe.Pointer(&hPrinter)),
		0,
	)
	if r1 == 0 {
		return fmt.Errorf("OpenPrinter falló para %q: %v", printerName, e1)
	}
	defer procClosePrinter.Call(uintptr(hPrinter))

	docName, _ := windows.UTF16PtrFromString("FprinterAgent Ticket")
	dataType, _ := windows.UTF16PtrFromString("RAW")
	di := docInfo1{
		DocName:    docName,
		OutputFile: nil,
		Datatype:   dataType,
	}

	r1, _, e1 = procStartDocPrinterW.Call(
		uintptr(hPrinter),
		1,
		uintptr(unsafe.Pointer(&di)),
	)
	if r1 == 0 {
		return fmt.Errorf("StartDocPrinter falló: %v", e1)
	}
	defer procEndDocPrinter.Call(uintptr(hPrinter))

	r1, _, e1 = procStartPagePrinter.Call(uintptr(hPrinter))
	if r1 == 0 {
		return fmt.Errorf("StartPagePrinter falló: %v", e1)
	}
	defer procEndPagePrinter.Call(uintptr(hPrinter))

	var written uint32
	r1, _, e1 = procWritePrinter.Call(
		uintptr(hPrinter),
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
		uintptr(unsafe.Pointer(&written)),
	)
	if r1 == 0 {
		return fmt.Errorf("WritePrinter falló: %v", e1)
	}
	if int(written) != len(data) {
		return fmt.Errorf("WritePrinter escribió %d de %d bytes", written, len(data))
	}
	return nil
}

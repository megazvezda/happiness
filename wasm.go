//go:build js && wasm

package main

import (
	"syscall/js"
)

func convert(this js.Value, args []js.Value) any {
	if len(args) != 1 {
		return js.ValueOf(map[string]any{
			"error": "expected one PDF byte array",
		})
	}

	pdfBytes := make([]byte, args[0].Length())
	js.CopyBytesToGo(pdfBytes, args[0])

	icsBytes, err := doConversion(pdfBytes)
	if err != nil {
		return js.ValueOf(map[string]any{
			"error": err.Error(),
		})
	}

	return js.ValueOf(string(icsBytes))
}

func main() {
	js.Global().Set("convertTimetable", js.FuncOf(convert))
	select {}
}

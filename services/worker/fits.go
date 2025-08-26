package main

// #cgo CFLAGS: -I/opt/homebrew/include/
// #cgo LDFLAGS: -L/opt/homebrew/lib/ -lcfitsio
// #include "test.h"
// #include <stdio.h> // needed for fflush and stdout
import "C"

func testFits(filename string) {
	C.printHeader(C.CString(filename))
	C.fflush(C.stdout)
}

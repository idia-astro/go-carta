package main

// #cgo pkg-config: cfitsio
// #include "test.h"
// #include <stdio.h> // needed for fflush and stdout
import "C"

func testFits(filename string) {
	C.printHeader(C.CString(filename))
	C.fflush(C.stdout)
}

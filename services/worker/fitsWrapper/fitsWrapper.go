package fitsWrapper

import (
	"fmt"
	"io"
	"log"

	helpers "idia-astro/go-carta/pkg/shared"
)

// #cgo pkg-config: cfitsio
// #include "test.h"
// #include "fitsio.h"
// #include <stdio.h> // needed for fflush and stdout
import "C"

type FitsFile struct {
	filePtr *C.fitsfile
	io.Closer
	numKeys int
}

func OpenFitsFile(filename string) (*FitsFile, error) {
	var status C.int
	var filePtr *C.fitsfile
	C.ffopen(&filePtr, C.CString(filename), C.READONLY, &status)
	if status != 0 {
		return nil, fmt.Errorf("failed to open file: %d", status)
	}
	log.Printf("File opened with status %v", status)
	return &FitsFile{filePtr: filePtr}, nil
}

func (f *FitsFile) Close() error {
	if f.filePtr == nil {
		return nil
	}

	var status C.int
	C.ffclos(f.filePtr, &status)
	if status != 0 {
		return fmt.Errorf("failed to close file: %d", status)
	}
	log.Printf("File closed with status %v", status)
	f.filePtr = nil
	return nil
}

func (f *FitsFile) GetNumHeaderKeys() (int, error) {
	if f.filePtr == nil {
		return -1, fmt.Errorf("file not opened")
	}

	if f.numKeys > 0 {
		return f.numKeys, nil
	}

	var nKeys C.int
	var status C.int
	C.ffghsp(f.filePtr, &nKeys, nil, &status)
	if status != 0 {
		return -1, fmt.Errorf("failed to open file: %d", status)
	}

	f.numKeys = int(nKeys)
	return f.numKeys, nil
}

// Note: Zero-indexed, while fits_read_record is 1-indexed

func (f *FitsFile) ReadHeader(i int) (string, error) {
	if f.filePtr == nil {
		return "", fmt.Errorf("file not opened")
	}

	n, err := f.GetNumHeaderKeys()

	if err != nil {
		return "", fmt.Errorf("failed to open file: %d", n)
	}

	if i > n || i < 0 {
		return "", fmt.Errorf("invalid header index: %d", i)
	}

	arr := [80]C.char{}
	var status C.int

	C.ffgrec(f.filePtr, C.int(i+1), (*C.char)(&arr[0]), &status)

	if status != 0 {
		return "", fmt.Errorf("failed to read header: %d", status)
	}
	return C.GoString((*C.char)(&arr[0])), nil
}

func TestWrapper(filename string) {
	f, err := OpenFitsFile(filename)
	if err != nil {
		log.Printf("failed to open file: %s", err)
		return
	}
	defer helpers.CloseOrLog(f)

	n, err := f.GetNumHeaderKeys()
	if err != nil {
		log.Printf("failed to get number of keys: %s", err)
		return
	}
	log.Printf("Printing %d header entries:", n)

	for i := 0; i < n; i++ {
		header, err := f.ReadHeader(i)
		if err != nil {
			log.Printf("failed to read header %d: %s", i, err)
			continue
		}
		log.Printf("%s", header)
	}

	C.fflush(C.stdout)
}

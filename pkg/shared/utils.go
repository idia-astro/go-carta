package helpers

import (
	"io"
	"log/slog"
)

// CloseOrLog Helper function to attempt to close IO connections and log error if it fails, useful for closing on defer
func CloseOrLog(closer io.Closer) {
	err := closer.Close()
	if err != nil {
		slog.Error("Error closing I/O", "error", err)
	}
}

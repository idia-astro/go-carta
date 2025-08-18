package httpHelpers

import (
	"encoding/json"
	"log"
	"net/http"
)

func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]any{"msg": msg})
}

func WriteOutput(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusOK, data)
}

func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		log.Printf("Error encoding JSON: %v", err)
		http.Error(w, "Error encoding JSON", http.StatusInternalServerError)
	}
}

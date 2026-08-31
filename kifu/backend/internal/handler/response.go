package handler

import (
	"encoding/json"
	"net/http"
)

// respondWithJSON writes a JSON response with the given status code and payload
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(response)
}

// respondWithError writes a JSON error response with the given status code and error message
func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

// isValidHexColor verifies if a color string is a valid 3-digit or 6-digit hex color (e.g. #fff or #4e342e)
func isValidHexColor(c string) bool {
	if len(c) != 4 && len(c) != 7 {
		return false
	}
	if c[0] != '#' {
		return false
	}
	for _, ch := range c[1:] {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return false
		}
	}
	return true
}

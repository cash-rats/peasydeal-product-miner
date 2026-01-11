package render

import (
	"encoding/json"
	"net/http"
)

func ChiJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func ChiErr(w http.ResponseWriter, status int, msg string) {
	ChiJSON(w, status, map[string]any{
		"error": msg,
	})
}

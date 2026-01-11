package render

import (
	"encoding/json"
	"net/http"
)

type errResponse struct {
	Error string `json:"error"`
}

func ChiJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func ChiErr(w http.ResponseWriter, r *http.Request, status int, err error) {
	msg := "unknown error"
	if err != nil {
		msg = err.Error()
	}
	ChiJSON(w, r, status, errResponse{Error: msg})
}

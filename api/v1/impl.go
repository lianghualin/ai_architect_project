package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Server struct{}

func NewServer() Server {
	return Server{}
}

// GetHello implements ServerInterface.
// (GET /hello)
func (Server) GetHello(w http.ResponseWriter, r *http.Request, params GetHelloParams) {
	resp := HelloResponse{
		Message: fmt.Sprintf("Hello, World %s", params.Name),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// Ensure Server implements ServerInterface
var _ ServerInterface = (*Server)(nil)

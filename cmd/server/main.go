package main

import (
	"log"
	"net/http"
	"os"

	api "example.com/demo-openapi/api/v1"
)

func main() {
	server := api.NewServer()

	mux := http.NewServeMux()
	api.HandlerFromMux(server, mux)

	// 托管 OpenAPI spec
	mux.HandleFunc("/api/v1/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		content, err := os.ReadFile("api/v1/openapi.yaml")
		if err != nil {
			http.Error(w, "Failed to read openapi.yaml", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/x-yaml")
		w.Write(content)
	})

	// 托管 Swagger UI
	mux.Handle("/docs/", http.StripPrefix("/docs/", http.FileServer(http.Dir("docs/swagger-ui"))))

	addr := "0.0.0.0:8080"
	log.Printf("Server starting on http://%s", addr)
	log.Printf("API: curl 'http://localhost:8080/hello?name=test'")
	log.Printf("Swagger UI: http://localhost:8080/docs/")

	s := &http.Server{
		Handler: mux,
		Addr:    addr,
	}

	log.Fatal(s.ListenAndServe())
}

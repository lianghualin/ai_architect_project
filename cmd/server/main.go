package main

import (
	"log"
	"net/http"
	"os"

	api "example.com/demo-openapi/api/v1"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// API key is now available via os.Getenv("API_KEY")
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		log.Println("Warning: API_KEY not set")
	}

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

	// CORS middleware
	corsHandler := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}

	s := &http.Server{
		Handler: corsHandler(mux),
		Addr:    addr,
	}

	log.Fatal(s.ListenAndServe())
}

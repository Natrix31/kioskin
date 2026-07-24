package main

import (
	_ "embed"
	"log"
	"net/http"

	swaggerFiles "github.com/swaggo/files"
)

//go:embed openapi.yaml
var openAPISpec []byte

const swaggerInitializerJS = `window.onload = function() {
  window.ui = SwaggerUIBundle({
    url: "/openapi.yaml",
    dom_id: "#swagger-ui",
    deepLinking: true,
    presets: [
      SwaggerUIBundle.presets.apis,
      SwaggerUIStandalonePreset
    ],
    plugins: [
      SwaggerUIBundle.plugins.DownloadUrl
    ],
    layout: "StandaloneLayout"
  });
};
`

func registerSwaggerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/openapi.yaml", handleOpenAPISpec)
	mux.HandleFunc("/docs", handleSwaggerDocs)
	mux.HandleFunc("/swagger/swagger-initializer.js", handleSwaggerInitializer)
	mux.Handle("/swagger/", http.StripPrefix("/swagger/", swaggerFiles.Handler))
}

func handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(openAPISpec); err != nil {
		log.Printf("Не удалось записать ответ /openapi.yaml: %v", err)
	}
}

func handleSwaggerDocs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	http.Redirect(w, r, "/swagger/index.html", http.StatusFound)
}

func handleSwaggerInitializer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(swaggerInitializerJS)); err != nil {
		log.Printf("Не удалось записать swagger-initializer.js: %v", err)
	}
}

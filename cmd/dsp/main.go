package main

import (
    "log"
    "net/http"
    "os"
    "time"

    "github.com/radiusdt/vector-dsp/internal/httpserver"
)

func main() {
    // Read bind address from environment or use default.
    addr := getEnv("VECTOR_DSP_HTTP_ADDR", ":8080")

    // Initialize HTTP server with our DSP handlers.
    srv := &http.Server{
        Addr:              addr,
        Handler:           httpserver.NewServer(),
        ReadHeaderTimeout: 2 * time.Second,
        ReadTimeout:       5 * time.Second,
        WriteTimeout:      5 * time.Second,
        MaxHeaderBytes:    1 << 20,
    }

    log.Printf("VectorDSP starting on %s", addr)
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("server error: %v", err)
    }
}

// getEnv returns the value of the environment variable or the default when empty.
func getEnv(key, def string) string {
    if v, ok := os.LookupEnv(key); ok && v != "" {
        return v
    }
    return def
}
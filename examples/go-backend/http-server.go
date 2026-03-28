package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Request representa una petición HTTP
type HTTPRequest struct {
	Method string      `json:"method"`
	Params interface{} `json:"params,omitempty"`
}

// HTTPResponse representa una respuesta HTTP
type HTTPResponse struct {
	Result interface{} `json:"result,omitempty"`
	Error  *Error      `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type HTTPServer struct {
	port int
}

func NewHTTPServer(port int) *HTTPServer {
	return &HTTPServer{port: port}
}

func (s *HTTPServer) handleAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req HTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var resp HTTPResponse

	switch req.Method {
	case "ping":
		resp.Result = map[string]interface{}{
			"status":    "pong",
			"timestamp": time.Now().Unix(),
		}

	case "processFile":
		params, ok := req.Params.(map[string]interface{})
		if !ok {
			resp.Error = &Error{
				Code:    -32602,
				Message: "Invalid params",
			}
		} else {
			filePath, _ := params["filePath"].(string)
			resp.Result = map[string]interface{}{
				"filePath": filePath,
				"processed": true,
			}
		}

	default:
		resp.Error = &Error{
			Code:    -32601,
			Message: fmt.Sprintf("Method not found: %s", req.Method),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "OK")
}

func (s *HTTPServer) Start() {
	http.HandleFunc("/api", s.handleAPI)
	http.HandleFunc("/health", s.handleHealth)

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Server starting on %s", addr)

	// Manejar señales
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down server...")
		os.Exit(0)
	}()

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}

func main() {
	port := 8080
	if len(os.Args) > 2 && os.Args[1] == "--port" {
		fmt.Sscanf(os.Args[2], "%d", &port)
	}

	server := NewHTTPServer(port)
	server.Start()
}











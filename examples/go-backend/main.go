package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Request representa una petición del cliente TypeScript
type Request struct {
	ID     string      `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params,omitempty"`
}

// Response representa una respuesta al cliente TypeScript
type Response struct {
	ID     string      `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *Error      `json:"error,omitempty"`
}

// Error representa un error en la respuesta
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Backend maneja las peticiones
type Backend struct {
	scanner *bufio.Scanner
	writer  io.Writer
}

func NewBackend() *Backend {
	return &Backend{
		scanner: bufio.NewScanner(os.Stdin),
		writer:  os.Stdout,
	}
}

func (b *Backend) handleRequest(req *Request) *Response {
	switch req.Method {
	case "ping":
		return &Response{
			ID:     req.ID,
			Result: map[string]interface{}{"status": "pong", "timestamp": time.Now().Unix()},
		}

	case "processFile":
		params, ok := req.Params.(map[string]interface{})
		if !ok {
			return &Response{
				ID: req.ID,
				Error: &Error{
					Code:    -32602,
					Message: "Invalid params",
				},
			}
		}
		filePath, _ := params["filePath"].(string)
		return b.processFile(req.ID, filePath)

	case "analyzeCode":
		params, ok := req.Params.(map[string]interface{})
		if !ok {
			return &Response{
				ID: req.ID,
				Error: &Error{
					Code:    -32602,
					Message: "Invalid params",
				},
			}
		}
		code, _ := params["code"].(string)
		return b.analyzeCode(req.ID, code)

	default:
		return &Response{
			ID: req.ID,
			Error: &Error{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}
}

func (b *Backend) processFile(id, filePath string) *Response {
	// Simular procesamiento de archivo
	// En un caso real, aquí procesarías el archivo
	info, err := os.Stat(filePath)
	if err != nil {
		return &Response{
			ID: id,
			Error: &Error{
				Code:    -32000,
				Message: err.Error(),
			},
		}
	}

	return &Response{
		ID: id,
		Result: map[string]interface{}{
			"filePath": filePath,
			"size":     info.Size(),
			"modified": info.ModTime().Unix(),
			"processed": true,
		},
	}
}

func (b *Backend) analyzeCode(id, code string) *Response {
	// Simular análisis de código
	// En un caso real, usarías herramientas como go/parser, go/ast, etc.
	lines := 0
	for _, char := range code {
		if char == '\n' {
			lines++
		}
	}

	return &Response{
		ID: id,
		Result: map[string]interface{}{
			"lines":      lines,
			"characters": len(code),
			"analyzed":   true,
		},
	}
}

func (b *Backend) sendResponse(resp *Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Error marshaling response: %v", err)
		return
	}
	fmt.Fprintf(b.writer, "%s\n", string(data))
}

func (b *Backend) Run() {
	// Manejar señales para cierre graceful
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down...")
		os.Exit(0)
	}()

	log.Println("Backend started, waiting for requests...")

	for b.scanner.Scan() {
		line := b.scanner.Text()
		if line == "" {
			continue
		}

		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			log.Printf("Error parsing request: %v", err)
			continue
		}

		resp := b.handleRequest(&req)
		b.sendResponse(resp)
	}

	if err := b.scanner.Err(); err != nil {
		log.Printf("Error reading input: %v", err)
	}
}

func main() {
	backend := NewBackend()
	backend.Run()
}











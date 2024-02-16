package main

import (
	"errors"
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"
)

// Constants
const (
	EVN_OPENAI_API_KEY = "OPENAI_API_KEY"
	HOST_OPENAI_API    = "api.openai.com"
)

// Variables
var (
	port               int                     // Port number to listen on
	trueKey            string                  // Real OpenAI API key
	virtualKeyFilePath string                  // File path for virtual OpenAI keys
	virtualKeys        = make(map[string]bool) // Map to store virtual OpenAI keys
)

// init function to initialize command-line flags
func init() {
	// Define command-line flags
	flag.IntVar(&port, "port", 48080, "Port number to listen on")
	flag.StringVar(&virtualKeyFilePath, "vkeys", "virtual_api_keys.txt", "File path for virtual OpenAI keys")
}

// config function to get the real OpenAI API key from the environment variables and read virtual API keys from a file
func config() {
	// Get the real OpenAI API key from the environment variables
	trueKey = os.Getenv(EVN_OPENAI_API_KEY)
	if trueKey == "" {
		log.Fatal(errors.New(EVN_OPENAI_API_KEY + " environment variable is not defined"))
	}

	// Log the loading of virtual API keys from a file
	log.Printf("*** load virtual api keys from %s", virtualKeyFilePath)

	// Read virtual API keys from the specified file
	content, err := os.ReadFile(virtualKeyFilePath)
	if err != nil {
		log.Fatal("Error reading file:", err)
	}

	// Populate the virtualKeys map with the read keys
	for _, line := range strings.Split(string(content), "\n") {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine != "" {
			virtualKeys[trimmedLine] = true
		}
	}
}

// ReverseProxyHandler handles incoming HTTP requests and forwards them to the OpenAI API with proper authentication
func ReverseProxyHandler(w http.ResponseWriter, r *http.Request) {
	// Log information about the incoming request
	log.Printf("*** request from %s with auth: %s\n", r.RemoteAddr, r.Header.Get("Authorization"))

	// Set the target OpenAI API and initialize the key variable
	target := HOST_OPENAI_API
	key := ""

	// Extract the key from the Authorization header
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		key = strings.TrimPrefix(authHeader, "Bearer ")
	}

	// Check if the key is a virtual key, if not, use the real key
	if _, exists := virtualKeys[key]; exists {
		key = trueKey
	} else {
		log.Printf("****** Warning: No virtual key found")
	}

	// Set up the reverse proxy director function
	director := func(req *http.Request) {
		req.URL.Scheme = "https"
		req.URL.Host = target
		req.Host = target
		req.Header.Set("Authorization", "Bearer "+key)
	}

	// Create a reverse proxy and serve the HTTP request
	proxy := &httputil.ReverseProxy{Director: director}
	proxy.ServeHTTP(w, r)

	// Log information about the response headers
	log.Printf("*** response with header: %s\n", w.Header())
}

// main function to start the HTTP server
func main() {
	// Parse command-line flags
	flag.Parse()

	// Configure varoius keys
	config()

	// Log the start of the server
	log.Printf("*** start server: %v\n", port)

	// Start the HTTP server with the ReverseProxyHandler as the handler
	if err := http.ListenAndServe(":"+strconv.Itoa(port), http.HandlerFunc(ReverseProxyHandler)); err != nil {
		log.Fatal(err)
	}
}

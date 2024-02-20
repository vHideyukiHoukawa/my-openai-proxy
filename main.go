package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Constants
const (
	EVN_OPENAI_API_KEY = "OPENAI_API_KEY"
	HOST_OPENAI_API    = "api.openai.com"
)

// Variables
var (
	port               int                     // Port number to listen on
	accessCountLimit   int64                   // Total access count limit
	accessCounter      int64                   // Counter to track the total access count
	mu                 sync.Mutex              // Mutex to synchronize accessCounter updates
	realKey            string                  // Real OpenAI API key
	virtualKeyFilePath string                  // Path to the file containing virtual OpenAI API keys
	virtualKeys        = make(map[string]bool) // Map to store virtual OpenAI API keys
)

// init function to initialize command-line flags
func init() {
	// Define command-line flags
	flag.IntVar(&port, "port", 48080, "Port number to listen on.")
	flag.Int64Var(&accessCountLimit, "access-count-limit", -1, "Total access count limit. Use -1 for no limit.")
	flag.StringVar(&virtualKeyFilePath, "virtual-keys-file", "virtual-api-keys.txt", "Path to the file containing virtual OpenAI API keys.\nEach key should be specified on a separate line.")

	additionalHelp1 := `
This program offers reverse proxy functionality to the OpenAI API server with additional features, including:
  1. Abuse protection through a total access count limit.
  2. Enhanced security by using virtual API keys instead of exposing the real API key.
`
	additionalHelp2 := fmt.Sprintf(`
Note:
  * Set your real OpenAI API key as the environment variable: %s.
  * Configure your app's OpenAI API access to use http://<ip-or-hostname-of-this-machine>:<port>/v1.
    Ensure the path includes "/v1".
`, EVN_OPENAI_API_KEY)

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, additionalHelp1)
		fmt.Fprintf(os.Stderr, "\nUsage: %s [options]\n\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprint(os.Stderr, additionalHelp2)
	}
}

// config function to get the real OpenAI API key from the environment variables and read virtual API keys from a file
func config() {
	// Get the real OpenAI API key from the environment variables
	realKey = os.Getenv(EVN_OPENAI_API_KEY)
	if realKey == "" {
		log.Fatal(errors.New(EVN_OPENAI_API_KEY + " environment variable is not defined. Please set a real OpenAI API key for this"))
	}

	// Log the loading of virtual API keys from a file
	log.Printf("*** load virtual api keys from %s", virtualKeyFilePath)

	// Read virtual API keys from the specified file
	content, err := os.ReadFile(virtualKeyFilePath)
	if err != nil {
		log.Fatalf("****** Error: %s\nPlease provide virtual API keys in the file: %s, with each key on a separate line.", err, virtualKeyFilePath)
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

	// Increment the access counter while protecting it with a mutex
	mu.Lock()
	accessCounter++
	count := accessCounter
	mu.Unlock()

	// Log information about the incoming request
	// log.Printf("*** request No.%d from %s with auth: %s\n", count, r.RemoteAddr, r.Header.Get("Authorization"))
	log.Printf("*** request No.%d from %s\n", count, r.RemoteAddr)

	// Check total access limit and return error if exceeded
	if accessCountLimit != -1 && count > accessCountLimit {
		log.Printf("****** Warning: Total access limit of %d exceeded", accessCountLimit)
		http.Error(w, "Total access count limit exceeded.", http.StatusUnprocessableEntity)
		return
	}

	// Set the target OpenAI API and initialize the key variable
	target := HOST_OPENAI_API
	key := ""

	// Extract the key from the Authorization header
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		key = strings.TrimPrefix(authHeader, "Bearer ")
	}

	// Check if the key is a virtual key; if it is, replace it with the real key
	if _, exists := virtualKeys[key]; exists {
		key = realKey
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
	// log.Printf("*** response with header: %s\n", w.Header())
}

// main function to start the HTTP server
func main() {
	// Parse command-line flags
	flag.Parse()

	// Setup keys configuration
	config()

	// Log the start of the server
	log.Printf("*** start server: %v\n", port)

	// Start the HTTP server with the ReverseProxyHandler as the handler
	if err := http.ListenAndServe(":"+strconv.Itoa(port), http.HandlerFunc(ReverseProxyHandler)); err != nil {
		log.Fatal(err)
	}
}

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

const (
	EVN_OPENAI_API_KEY = "OPENAI_API_KEY"
	HOST_OPENAI_API    = "api.openai.com"
)

var (
	port                  int
	true_key              string
	virtual_key_file_path string
	virtualKeys           = make(map[string]bool)
)

func init() {
	flag.IntVar(&port, "port", 48080, "Listen port number")
	flag.StringVar(&virtual_key_file_path, "virtual_key_file_path", "virtual_api_keys.txt", "A file containing virtual OpenAI keys")
	true_key = os.Getenv(EVN_OPENAI_API_KEY)
	if true_key == "" {
		log.Fatal(errors.New(EVN_OPENAI_API_KEY + " environment variable is not defined"))
	}

	log.Printf("*** load virtual api keys from %s", virtual_key_file_path)
	content, err := os.ReadFile(virtual_key_file_path)
	if err != nil {
		log.Fatal("Error reading file:", err)
	}

	for _, line := range strings.Split(string(content), "\n") {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine != "" {
			virtualKeys[trimmedLine] = true
		}
	}
}

func ReverseProxyHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("*** request from %s with auth: %s\n", r.RemoteAddr, r.Header.Get("Authorization"))
	target := HOST_OPENAI_API
	key := ""

	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		key = strings.TrimPrefix(authHeader, "Bearer ")
	}
	if _, exists := virtualKeys[key]; exists {
		key = true_key
	} else {
		log.Printf("****** Warning: No virtual key found")
	}

	director := func(req *http.Request) {
		req.URL.Scheme = "https"
		req.URL.Host = target
		req.Host = target
		req.Header.Set("Authorization", "Bearer "+key)
	}
	proxy := &httputil.ReverseProxy{Director: director}
	proxy.ServeHTTP(w, r)
	log.Printf("*** response with header: %s\n", w.Header())
}

func main() {
	flag.Parse()
	log.Printf("*** start server: %v\n", port)
	if err := http.ListenAndServe(":"+strconv.Itoa(port), http.HandlerFunc(ReverseProxyHandler)); err != nil {
		log.Fatal(err)
	}
}

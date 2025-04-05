package main

import (
	"crypto/md5"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// ETagFileServer is a custom file server that adds ETag support
type ETagFileServer struct {
	root http.FileSystem
}

// NewETagFileServer creates a new file server with ETag support
func NewETagFileServer(root http.FileSystem) *ETagFileServer {
	return &ETagFileServer{root}
}

// ServeHTTP implements the http.Handler interface
func (fs *ETagFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Clean the path
	upath := r.URL.Path
	if !strings.HasPrefix(upath, "/") {
		upath = "/" + upath
	}

	// Try to open the file
	f, err := fs.root.Open(upath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()

	// Get file info
	fi, err := f.Stat()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Handle directory
	if fi.IsDir() {
		// Redirect if the directory name doesn't end in a slash
		if r.URL.Path[len(r.URL.Path)-1] != '/' {
			http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
			return
		}

		// Try to open the index.html file
		index := filepath.Join(upath, "index.html")
		indexFile, err := fs.root.Open(index)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer indexFile.Close()

		// Get index.html info
		indexInfo, err := indexFile.Stat()
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Generate ETag for index.html
		etag := generateETag(indexInfo)
		w.Header().Set("ETag", etag)

		// Check If-None-Match
		if match := r.Header.Get("If-None-Match"); match != "" && match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		// Set Content-Type
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=0")
		http.ServeContent(w, r, indexInfo.Name(), indexInfo.ModTime(), indexFile)
		return
	}

	// Generate ETag for file
	etag := generateETag(fi)
	w.Header().Set("ETag", etag)

	// Check If-None-Match
	if match := r.Header.Get("If-None-Match"); match != "" && match == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	// Detect content type
	// We need to reset the file pointer after detecting the content type
	buffer := make([]byte, 512)
	_, err = f.Read(buffer)
	if err != nil && err != io.EOF {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	contentType := http.DetectContentType(buffer)

	// Reset file pointer
	f, err = fs.root.Open(upath)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	// Set appropriate headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=0")

	// Serve the file content
	http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
}

// generateETag creates an ETag based on file size, modification time, and name
func generateETag(fi os.FileInfo) string {
	// Create a unique identifier for the file
	h := md5.New()
	fmt.Fprintf(h, "%s:%d:%d", fi.Name(), fi.Size(), fi.ModTime().UnixNano())
	return fmt.Sprintf("%x", h.Sum(nil))
}

func main() {
	// Serve static files from current directory with ETag support
	fs := NewETagFileServer(http.Dir("."))
	http.Handle("/", fs)

	// Reverse proxy to api.anthropic.com
	apiURL, err := url.Parse("https://api.anthropic.com")
	if err != nil {
		log.Fatal(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(apiURL)

	proxy.Transport = http.DefaultTransport.(*http.Transport).Clone()
	proxy.Transport.(*http.Transport).Proxy = http.ProxyFromEnvironment

	proxy.Director = func(req *http.Request) {
		// pull origin, cookies

		req.Header.Del("Origin")
		req.Header.Del("Cookie")
		req.Header.Del("Referrer")
		// remove "Sec-" headers:
		for k := range req.Header {
			if len(k) > 4 && k[:4] == "Sec-" {
				req.Header.Del(k)
			}
		}

		// set the request URL to the target URL
		req.URL.Scheme = apiURL.Scheme
		req.URL.Host = apiURL.Host
		req.Host = apiURL.Host
		// set the request path to the target URL
		o, _ := httputil.DumpRequestOut(req, true)
		fmt.Println(string(o))
	}
	http.Handle("/v1/", proxy)

	//openBrowser("http://localhost:8081")
	// Start the API proxy server
	log.Println("Starting cgpt-serve on :8081")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatal(err)
	}
}

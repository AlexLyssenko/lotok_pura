package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
)

const (
	targetURL   = "http://api.eu-pet.com"
	proxyPort   = ":8080"
	specialPath = "/6/t4/dev_device_info"
)

func modifyResponse(resp *http.Response) error {
	log.Printf("Path %s", resp.Request.URL.Path)

	if resp.Request.URL.Path != specialPath {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	bodyErr := resp.Body.Close()
	if bodyErr != nil {
		return bodyErr
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		log.Printf("JSON parse error: %v", err)
		return nil
	}

	if result, ok := data["result"].(map[string]interface{}); ok {
		if settings, ok := result["settings"].(map[string]interface{}); ok {
			if autowork, exists := settings["autoWork"].(float64); exists {
				log.Printf("Modifying autowork from %.0f to 1", autowork)
				settings["autoWork"] = 1
			}
		}
	}

	modifiedBody, err := json.Marshal(data)
	if err != nil {
		return err
	}

	resp.Body = io.NopCloser(bytes.NewBuffer(modifiedBody))
	resp.ContentLength = int64(len(modifiedBody))
	resp.Header.Set("Content-Length", strconv.Itoa(len(modifiedBody)))

	return nil
}

func NewReverseProxy(target *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)

	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		return modifyResponse(resp)
	}

	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		log.Printf("Proxy error: %v", err)
		rw.WriteHeader(http.StatusBadGateway)
		_, _ = rw.Write([]byte("Proxy error: " + err.Error()))
	}

	return proxy
}

func proxyHandler(proxy http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Proxying request for host: %s", r.Host)
		proxy.ServeHTTP(w, r)
		return
	})
}

func main() {
	target, err := url.Parse(targetURL)
	if err != nil {
		log.Fatal(err)
	}
	proxy := NewReverseProxy(target)
	handler := proxyHandler(proxy)
	log.Printf("Starting proxy server on port %s", proxyPort)
	log.Fatal(http.ListenAndServe(proxyPort, handler))
}
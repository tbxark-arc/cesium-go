package main

import (
	"bytes"
	"cesium-go/assets"
	"encoding/json"
	"fmt"
	"github.com/dgraph-io/badger/v4"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"time"
)

type CacheEntry struct {
	Headers http.Header `json:"headers"`
	Body    []byte      `json:"body"`
}

type customResponseWriter struct {
	http.ResponseWriter
	statusCode int
	buffer     bytes.Buffer
}

func (w *customResponseWriter) Write(b []byte) (int, error) {
	w.buffer.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *customResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func newHttpServerWithReverseProxy(address string, cache *badger.DB, proxy *httputil.ReverseProxy) (*http.Server, error) {

	indexPath := map[string]struct{}{
		"/":           {},
		"":            {},
		"/index.html": {},
	}

	return &http.Server{
		Addr: address,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				w.WriteHeader(http.StatusOK)
				return
			}
			key := r.URL.Path
			if _, ok := indexPath[key]; ok {
				w.Header().Set("Content-Type", "text/html")
				_, _ = w.Write(assets.IndexHTML)
				return
			}
			if r.Method == http.MethodGet {
				err := cache.View(func(txn *badger.Txn) error {
					item, err := txn.Get([]byte(key))
					if err != nil {
						return err
					}
					var entry CacheEntry
					err = item.Value(func(val []byte) error {
						return json.Unmarshal(val, &entry)
					})
					log.Printf("cache hit: %s", key)
					if err != nil {
						return err
					}
					for k, v := range entry.Headers {
						for _, vv := range v {
							w.Header().Add(k, vv)
						}
					}
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(entry.Body)
					return nil
				})
				if err == nil {
					return
				}
			}
			proxy.ServeHTTP(w, r)
		}),
		ReadTimeout:  15 * time.Minute,
		WriteTimeout: 15 * time.Minute,
		IdleTimeout:  60 * time.Minute,
	}, nil
}

func newReverseProxyWithCache(keys []string, cache *badger.DB) (*httputil.ReverseProxy, error) {
	target, err := url.Parse("https://tile.googleapis.com/")
	if err != nil {
		return nil, err
	}
	index := atomic.Int64{}
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
		if len(keys) > 0 {
			key := keys[index.Add(1)%int64(len(keys))]
			query := req.URL.Query()
			query.Set("key", key)
			req.URL.RawQuery = query.Encode()
		}
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		resp.ContentLength = int64(len(bodyBytes))
		resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(bodyBytes)))
		if resp.StatusCode == http.StatusOK {
			entry := CacheEntry{
				Headers: http.Header{},
				Body:    bodyBytes,
			}
			for k, v := range resp.Header {
				for _, vv := range v {
					entry.Headers.Add(k, vv)
				}
			}
			_ = cache.Update(func(txn *badger.Txn) error {
				raw, err := json.Marshal(entry)
				if err != nil {
					return err
				}
				return txn.Set([]byte(resp.Request.URL.Path), raw)
			})

		}
		return nil
	}
	return proxy, nil
}

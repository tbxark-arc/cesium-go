package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/dgraph-io/badger/v4"
)

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
				_, _ = w.Write(IndexHTML)
				return
			}
			if r.Method == http.MethodGet {
				if entry, err := loadCache(cache, key); err == nil {
					log.Printf("load cache %s", key)
					for k, v := range entry.Header {
						for _, vv := range v {
							w.Header().Add(k, vv)
						}
					}
					w.Header().Set("Content-Length", fmt.Sprintf("%d", len(entry.Body)))
					_, _ = w.Write(entry.Body)
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
	target, _ := url.Parse("https://tile.googleapis.com/")
	index := atomic.Uint64{}
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
		req.Header.Del("Accept-Encoding")
		if len(keys) > 0 {
			key := keys[index.Add(1)%uint64(len(keys))]
			query := req.URL.Query()
			query.Set("key", key)
			req.URL.RawQuery = query.Encode()
		}
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.StatusCode != http.StatusOK {
			return nil
		}
		if resp.Request.Method != http.MethodGet {
			return nil
		}
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		resp.ContentLength = int64(len(bodyBytes))
		resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(bodyBytes)))
		entry := &CacheEntry{
			Header: resp.Header.Clone(),
			Body:   bodyBytes,
		}
		key := resp.Request.URL.Path
		err = saveCache(cache, key, entry)
		if err != nil {
			log.Printf("save cache error: %v", err)
			return nil
		}
		log.Printf("save cache %s", key)
		return nil
	}
	return proxy, nil
}

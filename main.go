package main

import (
	"errors"
	"flag"
	"github.com/TBXark/confstore"
	"github.com/dgraph-io/badger/v4"
	"log"
	"net/http"
)

type Config struct {
	Address  string   `json:"address"`
	CacheDir string   `json:"cache_dir"`
	APIKeys  []string `json:"api_keys"`
}

type Application struct {
	config *Config
	cache  *badger.DB
	server *http.Server
}

func NewApplication(config *Config) (*Application, error) {
	cacheOptions := badger.DefaultOptions(config.CacheDir)
	cache, err := badger.Open(cacheOptions)
	if err != nil {
		return nil, err
	}
	proxy, err := newReverseProxyWithCache(config.APIKeys, cache)
	if err != nil {
		return nil, err
	}
	server, err := newHttpServerWithReverseProxy(config.Address, cache, proxy)
	if err != nil {
		return nil, err
	}
	return &Application{
		config: config,
		cache:  cache,
		server: server,
	}, nil
}

func (p *Application) Start() error {
	log.Printf("Starting server on %s", p.config.Address)
	return p.server.ListenAndServe()
}

func (p *Application) Stop() error {
	return errors.Join(
		p.cache.Close(),
		p.server.Close(),
	)
}

func main() {
	conf := flag.String("config", "config.json", "Config file path")
	flag.Parse()
	config, err := confstore.Load[Config](*conf)
	if err != nil {
		log.Fatal(err)
	}
	proxy, err := NewApplication(config)
	if err != nil {
		log.Fatal(err)
	}
	err = proxy.Start()
	if err != nil {
		log.Fatal(err)
	}
}

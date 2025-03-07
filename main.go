package main

import (
	"errors"
	"flag"
	"github.com/TBXark/confstore"
	"github.com/dgraph-io/badger/v4"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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

func onsShutdown(closer ...func() error) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	log.Printf("Receive close signal")
	for _, c := range closer {
		err := c()
		if err != nil {
			log.Printf("close error: %v", err)
		}
	}
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
	go onsShutdown(proxy.Stop)
	err = proxy.Start()
	if err != nil {
		log.Fatal(err)
	}
}

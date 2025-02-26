package main

import (
	"encoding/json"
	"github.com/dgraph-io/badger/v4"
	"net/http"
)

type CacheEntry struct {
	Header http.Header
	Body   []byte
}

const (
	CacheTypeKeyHeader = "header"
	CacheTypeKeyBody   = "body"
)

func cacheKeys(t, uri string) ([]byte, error) {
	return json.Marshal([]string{t, uri})
}

func loadCache(db *badger.DB, uri string) (*CacheEntry, error) {
	var entry CacheEntry
	err := db.View(func(txn *badger.Txn) error {
		headerKey, err := cacheKeys(CacheTypeKeyHeader, uri)
		if err != nil {
			return err
		}
		headerRaw, err := txn.Get(headerKey)
		if err != nil {
			return err
		}
		bodyKey, err := cacheKeys(CacheTypeKeyBody, uri)
		if err != nil {
			return err
		}
		bodyRaw, err := txn.Get(bodyKey)
		if err != nil {
			return err
		}
		err = headerRaw.Value(func(val []byte) error {
			return json.Unmarshal(val, &entry.Header)
		})
		if err != nil {
			return err
		}
		err = bodyRaw.Value(func(val []byte) error {
			entry.Body = val
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	return &entry, err
}

func saveCache(db *badger.DB, uri string, entry *CacheEntry) error {
	return db.Update(func(txn *badger.Txn) error {
		headerKey, err := cacheKeys(CacheTypeKeyHeader, uri)
		if err != nil {
			return err
		}
		headerRaw, err := json.Marshal(entry.Header)
		if err != nil {
			return err
		}
		err = txn.Set(headerKey, headerRaw)
		if err != nil {
			return err
		}
		bodyKey, err := cacheKeys(CacheTypeKeyBody, uri)
		if err != nil {
			return err
		}
		err = txn.Set(bodyKey, entry.Body)
		if err != nil {
			return err
		}
		return nil
	})
}

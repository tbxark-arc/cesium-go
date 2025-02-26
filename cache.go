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

func cacheKeys(key, uri string) ([]byte, error) {
	return json.Marshal([]string{key, uri})
}

func loadCache(db *badger.DB, uri string) (*CacheEntry, error) {
	var entry CacheEntry
	err := db.View(func(txn *badger.Txn) error {
		headerRaw, err := loadCacheRaw(txn, CacheTypeKeyHeader, uri)
		if err != nil {
			return err
		}
		bodyRaw, err := loadCacheRaw(txn, CacheTypeKeyBody, uri)
		if err != nil {
			return err
		}
		err = headerRaw.Value(func(val []byte) error {
			return json.Unmarshal(val, &entry.Header)
		})
		if err != nil {
			return err
		}
		body, err := bodyRaw.ValueCopy(entry.Body)
		if err != nil {
			return err
		}
		entry.Body = body
		return nil
	})
	return &entry, err
}

func saveCache(db *badger.DB, uri string, entry *CacheEntry) error {
	return db.Update(func(txn *badger.Txn) error {
		headerRaw, err := json.Marshal(entry.Header)
		if err != nil {
			return err
		}
		err = setCacheRaw(txn, CacheTypeKeyHeader, uri, headerRaw)
		if err != nil {
			return err
		}
		err = setCacheRaw(txn, CacheTypeKeyBody, uri, entry.Body)
		if err != nil {
			return err
		}
		return nil
	})
}

func loadCacheRaw(txn *badger.Txn, key, uri string) (*badger.Item, error) {
	headerKey, err := cacheKeys(key, uri)
	if err != nil {
		return nil, err
	}
	headerRaw, err := txn.Get(headerKey)
	if err != nil {
		return nil, err
	}
	return headerRaw, nil
}

func setCacheRaw(txn *badger.Txn, key, uri string, value []byte) error {
	headerKey, err := cacheKeys(key, uri)
	if err != nil {
		return err
	}
	err = txn.Set(headerKey, value)
	if err != nil {
		return err
	}
	return nil
}

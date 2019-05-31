package redis

import (
	"bytes"
	"encoding/gob"
	"sync"
)

type Store struct {
	lock   sync.RWMutex
	values map[interface{}]interface{}
}

func NewStore() *Store {
	return &Store{
		values: make(map[interface{}]interface{}),
	}
}

// Set value
func (rs *Store) Set(key, value interface{}) error {
	rs.lock.Lock()
	defer rs.lock.Unlock()
	rs.values[key] = value
	return nil
}

// Get value
func (rs *Store) Get(key interface{}) interface{} {
	rs.lock.RLock()
	defer rs.lock.RUnlock()
	if v, ok := rs.values[key]; ok {
		return v
	}
	return nil
}

// Delete value
func (rs *Store) Delete(key interface{}) error {
	rs.lock.Lock()
	defer rs.lock.Unlock()
	delete(rs.values, key)
	return nil
}

// Flush clear all values
func (rs *Store) Flush() error {
	rs.lock.Lock()
	defer rs.lock.Unlock()
	rs.values = make(map[interface{}]interface{})
	return nil
}

// Serialize using gob
func (rs *Store) Serialize() ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(rs.values)
	if err == nil {
		return buf.Bytes(), nil
	}
	return nil, err
}

// Deserialize back to map[interface{}]interface{}
func (rs *Store) Deserialize(d []byte) error {
	dec := gob.NewDecoder(bytes.NewBuffer(d))
	return dec.Decode(&rs.values)
}

// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runbook

import (
	"skygo/log"
)

// KV represents key-value state
type KV struct {
	name string // name of key-value

	// TODO: need lock  or use sync.Map ?
	vars map[string]interface{}
}

// KVGetter holds metholds to read key-value
type KVGetter interface {
	LookupVar(key string) (string, bool)
	Range(f func(key, value string))
}

// KVSetter holds method to configure key-value
type KVSetter interface {
	SetKv(key string, value interface{})
}

// Init initialize KV that must be called firstly
func (kv *KV) Init(name string) {
	kv.vars = make(map[string]interface{})
	kv.name = name
}

// Init2 initialize KV with external kv store
func (kv *KV) Init2(name string, vars map[string]interface{}) {
	kv.vars = vars
	kv.name = name
}

// GetVar return value of var key
func (kv *KV) GetVar(key string) string {
	if v, ok := kv.Get(key).(string); ok {
		return v
	}
	return ""
}

// GetVar return value of var key
func (kv *KV) Get(key string) interface{} {
	v, ok := kv.vars[key]
	if !ok {
		log.Warning("Key %s is not found in %s", key, kv.name)
	}
	return v
}

// Setreturn value of var key
func (kv *KV) SetKv(key string, value interface{}) {
	if _, ok := kv.vars[key]; ok {
		log.Warning("To overwrite key %s held by %s", key, kv.name)
	}
	kv.vars[key] = value
}

// LookupVar retrieves the value of the variable named by the key.
// If the variable is present, value (which may be empty) is returned
// and the boolean is true. Otherwise the returned value will be empty
// and the boolean will be false.
func (kv *KV) LookupVar(key string) (string, bool) {
	if v, ok := kv.Lookup(key); ok {
		return v.(string), true
	}

	return "", false
}

// Lookup retrieves the value of the variable named by the key.
// If the key is present, value is returned and the boolean is true.
// Otherwise the returned value will be nill and the boolean will be false.
func (kv *KV) Lookup(key string) (interface{}, bool) {

	value, ok := kv.vars[key]
	return value, ok
}

// Range interates each item of key-value
func (kv *KV) Range(f func(key, value string)) {

	for key, value := range kv.vars {
		if v, ok := value.(string); ok {
			f(key, v)
		}
	}
}

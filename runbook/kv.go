// Copyright © 2019 Michael. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runbook

import (
	"skygo/utils/log"
)

// KV represents key-value state
type KV struct {
	name string // name of key-value

	// TODO: need lock  or use sync.Map ?
	vars map[string]interface{}
}

// KVGetter holds metholds to read key-value
type KVGetter interface {
	Get(string) interface{}
	Range(f func(key, value string))
}

// KVSetter holds method to configure key-value
type KVSetter interface {
	Set(key string, value interface{})
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

// GetStr return value of var key
// if key is not found, return empty string
func (kv *KV) GetStr(key string) string {
	if v, ok := kv.Get(key).(string); ok {
		return v
	}
	return ""
}

// Get retrieve value of var key
// if not found, return nil
func (kv *KV) Get(key string) interface{} {
	v, ok := kv.vars[key]
	if !ok {
		log.Warning("Key %s is not found in %s", key, kv.name)
		return nil
	}
	return v
}

// Setreturn value of var key
func (kv *KV) Set(key string, value interface{}) {
	if _, ok := kv.vars[key]; ok {
		log.Warning("To overwrite key %s held by %s", key, kv.name)
	}
	kv.vars[key] = value
}

// Range interates each item of key-value
func (kv *KV) Range(f func(key, value string)) {

	for key, value := range kv.vars {
		if v, ok := value.(string); ok {
			f(key, v)
		}
	}
}

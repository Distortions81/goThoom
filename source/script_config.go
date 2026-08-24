package main

import (
	"reflect"
	"strings"
	"sync"
)

const scriptConfigStoragePrefix = "__config__:"

type scriptConfigEntry struct {
	Name         string
	Key          string
	Type         string
	Default      any
	Value        any
	Callback     any
	queue        *scriptEventQueue
	registration scriptRegistrationHandle
}

var (
	scriptConfigMu      sync.RWMutex
	scriptConfigEntries = map[string][]scriptConfigEntry{}
)

func scriptAddConfig(owner, name, typ string, args ...any) any {
	entry, ok := makeScriptConfigEntry(owner, name, typ, args...)
	if !ok {
		return nil
	}
	scriptRegisterConfig(owner, entry)
	return entry.Value
}

func makeScriptConfigEntry(owner, name, typ string, args ...any) (scriptConfigEntry, bool) {
	name = strings.TrimSpace(name)
	typ = normalizeScriptConfigType(typ)
	if name == "" || typ == "" {
		return scriptConfigEntry{}, false
	}
	key := strings.ToLower(strings.Join(strings.Fields(name), "_"))
	defaultValue := scriptConfigZeroValue(typ)
	var callback any
	if len(args) > 0 {
		if isScriptConfigCallback(args[0]) {
			callback = args[0]
		} else if value, ok := coerceScriptConfigValue(typ, args[0]); ok {
			defaultValue = value
		}
	}
	if len(args) > 1 && isScriptConfigCallback(args[1]) {
		callback = args[1]
	}
	value := defaultValue
	if stored := scriptStorageGet(owner, scriptConfigStoragePrefix+key); stored != nil {
		if converted, ok := coerceScriptConfigValue(typ, stored); ok {
			value = converted
		}
	}
	return scriptConfigEntry{
		Name:     name,
		Key:      key,
		Type:     typ,
		Default:  defaultValue,
		Value:    value,
		Callback: callback,
	}, true
}

func normalizeScriptConfigType(typ string) string {
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "bool", "boolean", "check-box", "checkbox":
		return "bool"
	case "int", "integer", "int-slider":
		return "int"
	case "float", "decimal", "float-slider":
		return "float"
	case "string", "text", "text-box":
		return "string"
	case "item", "item-selector":
		return "item"
	default:
		return ""
	}
}

func scriptConfigZeroValue(typ string) any {
	switch typ {
	case "bool":
		return false
	case "int":
		return 0
	case "float":
		return float64(0)
	case "string", "item":
		return ""
	default:
		return nil
	}
}

func coerceScriptConfigValue(typ string, value any) (any, bool) {
	switch typ {
	case "bool":
		v, ok := value.(bool)
		return v, ok
	case "int":
		switch v := value.(type) {
		case int:
			return v, true
		case int8:
			return int(v), true
		case int16:
			return int(v), true
		case int32:
			return int(v), true
		case int64:
			return int(v), true
		case float32:
			return int(v), true
		case float64:
			return int(v), true
		}
	case "float":
		switch v := value.(type) {
		case float32:
			return float64(v), true
		case float64:
			return v, true
		case int:
			return float64(v), true
		case int64:
			return float64(v), true
		}
	case "string", "item":
		v, ok := value.(string)
		return v, ok
	}
	return nil, false
}

func isScriptConfigCallback(callback any) bool {
	return callback != nil && reflect.TypeOf(callback).Kind() == reflect.Func
}

func scriptRegisterConfig(owner string, entry scriptConfigEntry) {
	entry.queue = currentScriptEventQueue(owner)
	scriptConfigMu.Lock()
	var oldRegistration scriptRegistrationHandle
	for _, existing := range scriptConfigEntries[owner] {
		if existing.Key == entry.Key {
			oldRegistration = existing.registration
			break
		}
	}
	scriptConfigMu.Unlock()
	oldRegistration.release()
	var registration scriptRegistrationHandle
	registration = registerScriptResource(owner, func() {
		scriptConfigMu.Lock()
		entries := scriptConfigEntries[owner]
		for i := len(entries) - 1; i >= 0; i-- {
			if entries[i].registration == registration {
				entries = append(entries[:i], entries[i+1:]...)
			}
		}
		if len(entries) == 0 {
			delete(scriptConfigEntries, owner)
		} else {
			scriptConfigEntries[owner] = entries
		}
		scriptConfigMu.Unlock()
		refreshscriptsWindow()
	})
	entry.registration = registration
	scriptConfigMu.Lock()
	entries := scriptConfigEntries[owner]
	scriptConfigEntries[owner] = append(entries, entry)
	scriptConfigMu.Unlock()
	refreshscriptsWindow()
}

func scriptSetConfigValue(owner, key string, value any) bool {
	scriptConfigMu.Lock()
	entries := scriptConfigEntries[owner]
	for i := range entries {
		if entries[i].Key != key {
			continue
		}
		converted, ok := coerceScriptConfigValue(entries[i].Type, value)
		if !ok {
			scriptConfigMu.Unlock()
			return false
		}
		entries[i].Value = converted
		callback := entries[i].Callback
		callbackQueue := entries[i].queue
		scriptConfigEntries[owner] = entries
		scriptConfigMu.Unlock()
		scriptStorageSet(owner, scriptConfigStoragePrefix+key, converted)
		savescriptStores()
		invokeScriptConfigCallback(owner, key, callbackQueue, callback, converted)
		return true
	}
	scriptConfigMu.Unlock()
	return false
}

func invokeScriptConfigCallback(owner, key string, eventQueue *scriptEventQueue, callback, value any) {
	if callback == nil {
		return
	}
	queueScriptCallbackWaitOn(eventQueue, owner, "Config "+key, func() {
		fn := reflect.ValueOf(callback)
		if fn.Kind() != reflect.Func || fn.Type().NumIn() != 1 {
			return
		}
		arg := reflect.ValueOf(value)
		want := fn.Type().In(0)
		if arg.Type().AssignableTo(want) {
			fn.Call([]reflect.Value{arg})
		} else if arg.Type().ConvertibleTo(want) {
			fn.Call([]reflect.Value{arg.Convert(want)})
		} else if want.Kind() == reflect.Interface && arg.Type().Implements(want) {
			fn.Call([]reflect.Value{arg})
		}
	})
}

func scriptRemoveConfig(owner string) {
	scriptConfigMu.Lock()
	entries := append([]scriptConfigEntry(nil), scriptConfigEntries[owner]...)
	scriptConfigMu.Unlock()
	for _, entry := range entries {
		entry.registration.release()
	}
	scriptConfigMu.Lock()
	delete(scriptConfigEntries, owner)
	scriptConfigMu.Unlock()
	refreshscriptsWindow()
}

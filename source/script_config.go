package main

import (
	"reflect"
	"strings"
	"sync"
	"unicode"

	scriptapi "gt2"
)

const scriptConfigStoragePrefix = "__config__:"

type scriptConfigEntry struct {
	Label        string
	Help         string
	Key          string
	Type         string
	Scope        string
	Default      any
	Value        any
	Callback     any
	Validate     any
	Choices      []string
	Min          float64
	Max          float64
	Step         float64
	queue        *scriptEventQueue
	registration scriptRegistrationHandle
}

func scriptConfigStorageKey(entry scriptConfigEntry) string {
	key := scriptConfigStoragePrefix + entry.Scope + ":"
	if entry.Scope == scriptapi.ScopeCharacter {
		key += strings.ToLower(strings.TrimSpace(playerName)) + ":"
	}
	return key + entry.Key
}

var (
	scriptConfigMu      sync.RWMutex
	scriptConfigEntries = map[string][]scriptConfigEntry{}
)

func makeTypedScriptConfigEntry(owner, key, label, help, scope, typ string, defaultValue, callback, validate any, choices []string, min, max, step float64) (scriptConfigEntry, bool) {
	key = strings.TrimSpace(key)
	label = strings.TrimSpace(label)
	help = strings.TrimSpace(help)
	if !validScriptOptionKey(key) {
		reportScriptCommandError(owner, "invalid setting key: "+key)
		return scriptConfigEntry{}, false
	}
	if label == "" {
		label = key
	}
	if scope == "" {
		scope = scriptapi.ScopeGlobal
	}
	if scope != scriptapi.ScopeGlobal && scope != scriptapi.ScopeCharacter {
		reportScriptCommandError(owner, "invalid setting scope: "+scope)
		return scriptConfigEntry{}, false
	}
	callback = nonNilScriptOptionFunc(callback)
	validate = nonNilScriptOptionFunc(validate)
	entry := scriptConfigEntry{
		Key: key, Label: label, Help: help, Type: typ, Scope: scope,
		Default: defaultValue, Value: defaultValue, Callback: callback, Validate: validate,
		Choices: append([]string(nil), choices...), Min: min, Max: max, Step: step,
	}
	if !scriptConfigValueValid(entry, defaultValue) {
		reportScriptCommandError(owner, "invalid default for setting "+key)
		return scriptConfigEntry{}, false
	}
	if stored := scriptStorageGet(owner, scriptConfigStorageKey(entry)); stored != nil {
		if converted, ok := coerceScriptConfigValue(typ, stored); ok && scriptConfigValueValid(entry, converted) {
			entry.Value = converted
		}
	}
	return entry, true
}

func nonNilScriptOptionFunc(fn any) any {
	if fn == nil {
		return nil
	}
	value := reflect.ValueOf(fn)
	if value.Kind() == reflect.Func && value.IsNil() {
		return nil
	}
	return fn
}

func validScriptOptionKey(key string) bool {
	if key == "" {
		return false
	}
	for index, char := range key {
		if unicode.IsLetter(char) || char == '_' || char == '-' || index > 0 && unicode.IsDigit(char) {
			continue
		}
		return false
	}
	return true
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
	case "text", "choice", "key", "item":
		v, ok := value.(string)
		return v, ok
	}
	return nil, false
}

func scriptConfigValueValid(entry scriptConfigEntry, value any) bool {
	switch entry.Type {
	case "int":
		v, ok := value.(int)
		if !ok || entry.Max > entry.Min && (float64(v) < entry.Min || float64(v) > entry.Max) {
			return false
		}
	case "float":
		v, ok := value.(float64)
		if !ok || entry.Max > entry.Min && (v < entry.Min || v > entry.Max) {
			return false
		}
	case "choice":
		v, ok := value.(string)
		if !ok {
			return false
		}
		found := false
		for _, choice := range entry.Choices {
			if v == choice {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	case "key":
		v, ok := value.(string)
		if !ok || !validScriptBindingText(v) {
			return false
		}
	}
	if entry.Validate == nil {
		return true
	}
	fn := reflect.ValueOf(entry.Validate)
	if fn.Kind() != reflect.Func || fn.Type().NumIn() != 1 || fn.Type().NumOut() != 1 || fn.Type().Out(0).Kind() != reflect.Bool {
		return false
	}
	arg := reflect.ValueOf(value)
	want := fn.Type().In(0)
	if !arg.IsValid() || !arg.Type().AssignableTo(want) {
		if !arg.IsValid() || !arg.Type().ConvertibleTo(want) {
			return false
		}
		arg = arg.Convert(want)
	}
	return fn.Call([]reflect.Value{arg})[0].Bool()
}

func validScriptBindingText(combo string) bool {
	parts := strings.Split(strings.TrimSpace(combo), "-")
	if len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return false
		}
	}
	return !isInputModifierName(parts[len(parts)-1])
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
	scriptConfigMu.RLock()
	var entry scriptConfigEntry
	found := false
	for _, candidate := range scriptConfigEntries[owner] {
		if candidate.Key == key {
			entry, found = candidate, true
			break
		}
	}
	scriptConfigMu.RUnlock()
	if !found {
		return false
	}
	converted, ok := coerceScriptConfigValue(entry.Type, value)
	if !ok {
		return false
	}
	valid := false
	if !queueScriptCallbackWaitOn(entry.queue, owner, "Validate setting "+key, func() {
		valid = scriptConfigValueValid(entry, converted)
	}) || !valid {
		return false
	}
	if reflect.DeepEqual(entry.Value, converted) {
		return true
	}
	scriptConfigMu.Lock()
	entries := scriptConfigEntries[owner]
	for index := range entries {
		if entries[index].Key == key {
			entries[index].Value = converted
			scriptConfigEntries[owner] = entries
			break
		}
	}
	scriptConfigMu.Unlock()
	scriptStorageSet(owner, scriptConfigStorageKey(entry), converted)
	savescriptStores()
	invokeScriptConfigCallback(owner, key, entry.queue, entry.Callback, converted)
	return true
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

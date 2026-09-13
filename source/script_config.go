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
	Character    string
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
		character := entry.Character
		if character == "" {
			character = playerName
		}
		key += strings.ToLower(strings.TrimSpace(character)) + ":"
	}
	return key + entry.Key
}

var (
	scriptConfigMu      sync.RWMutex
	scriptConfigEntries = map[string][]scriptConfigEntry{}
)

func makeTypedScriptConfigEntry(owner, key, label, help, scope, typ string, defaultValue, callback, validate any, choices []string, min, max, step float64) (scriptConfigEntry, bool) {
	return makeTypedScriptConfigEntryForCharacter(owner, playerName, key, label, help, scope, typ, defaultValue, callback, validate, choices, min, max, step)
}

func makeTypedScriptConfigEntryForCharacter(owner, character, key, label, help, scope, typ string, defaultValue, callback, validate any, choices []string, min, max, step float64) (scriptConfigEntry, bool) {
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
		Key: key, Label: label, Help: help, Type: typ, Scope: scope, Character: character,
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
	case "color":
		switch v := value.(type) {
		case uint32:
			return v, true
		case int:
			if v >= 0 {
				return uint32(v), true
			}
		case int64:
			if v >= 0 && uint64(v) <= uint64(^uint32(0)) {
				return uint32(v), true
			}
		case float64:
			if v >= 0 && v <= float64(^uint32(0)) && v == float64(uint32(v)) {
				return uint32(v), true
			}
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
	case "color":
		if _, ok := value.(uint32); !ok {
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
	scriptRegisterPrimaryConfig(owner, entry)
}

func scriptRegisterConfigForSession(session *Session, owner string, entry scriptConfigEntry) {
	if session == nil {
		scriptRegisterPrimaryConfig(owner, entry)
		return
	}
	entry.queue = currentSessionScriptEventQueue(session, owner)
	session.automation.scriptMu.Lock()
	var oldRegistration scriptRegistrationHandle
	for _, existing := range session.automation.scriptConfigs[owner] {
		if existing.Key == entry.Key {
			oldRegistration = existing.registration
			break
		}
	}
	session.automation.scriptMu.Unlock()
	oldRegistration.release()
	var registration scriptRegistrationHandle
	registration = registerSessionScriptResource(entry.queue, func() {
		session.automation.scriptMu.Lock()
		entries := session.automation.scriptConfigs[owner]
		for i := len(entries) - 1; i >= 0; i-- {
			if entries[i].registration == registration {
				entries = append(entries[:i], entries[i+1:]...)
			}
		}
		if len(entries) == 0 {
			delete(session.automation.scriptConfigs, owner)
		} else {
			session.automation.scriptConfigs[owner] = entries
		}
		session.automation.scriptMu.Unlock()
		refreshscriptsWindow()
	})
	entry.registration = registration
	session.automation.scriptMu.Lock()
	session.automation.scriptConfigs[owner] = append(session.automation.scriptConfigs[owner], entry)
	session.automation.scriptMu.Unlock()
	refreshscriptsWindow()
}

func scriptRegisterPrimaryConfig(owner string, entry scriptConfigEntry) {
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
	return scriptSetPrimaryConfigValue(owner, key, value)
}

func scriptSetConfigValueForSession(session *Session, owner, key string, value any) bool {
	if session == nil {
		return scriptSetPrimaryConfigValue(owner, key, value)
	}
	entries := session.scriptConfigEntriesSnapshot(owner)
	var entry scriptConfigEntry
	found := false
	for _, candidate := range entries {
		if candidate.Key == key {
			entry, found = candidate, true
			break
		}
	}
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
	storageKey := scriptConfigStorageKey(entry)
	scriptStorageSet(owner, storageKey, converted)
	savescriptStores()
	propagateScriptConfigValue(owner, storageKey, converted)
	return true
}

func scriptSetPrimaryConfigValue(owner, key string, value any) bool {
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
	storageKey := scriptConfigStorageKey(entry)
	scriptStorageSet(owner, storageKey, converted)
	savescriptStores()
	propagateScriptConfigValue(owner, storageKey, converted)
	return true
}

type scriptConfigCallback struct {
	key      string
	queue    *scriptEventQueue
	callback any
}

// propagateScriptConfigValue keeps every live interpreter that registered the
// same global or character-scoped storage key in sync. Each callback remains
// serialized on its owning session's event queue.
func propagateScriptConfigValue(owner, storageKey string, value any) {
	var callbacks []scriptConfigCallback
	scriptConfigMu.Lock()
	primaryEntries := scriptConfigEntries[owner]
	for index := range primaryEntries {
		if scriptConfigStorageKey(primaryEntries[index]) != storageKey || reflect.DeepEqual(primaryEntries[index].Value, value) {
			continue
		}
		primaryEntries[index].Value = value
		callbacks = append(callbacks, scriptConfigCallback{key: primaryEntries[index].Key, queue: primaryEntries[index].queue, callback: primaryEntries[index].Callback})
	}
	scriptConfigEntries[owner] = primaryEntries
	scriptConfigMu.Unlock()

	if appSessions != nil {
		for _, session := range appSessions.snapshot() {
			if session == nil || session.automation == nil {
				continue
			}
			session.automation.scriptMu.Lock()
			entries := session.automation.scriptConfigs[owner]
			for index := range entries {
				if scriptConfigStorageKey(entries[index]) != storageKey || reflect.DeepEqual(entries[index].Value, value) {
					continue
				}
				entries[index].Value = value
				callbacks = append(callbacks, scriptConfigCallback{key: entries[index].Key, queue: entries[index].queue, callback: entries[index].Callback})
			}
			session.automation.scriptConfigs[owner] = entries
			session.automation.scriptMu.Unlock()
		}
	}
	for _, callback := range callbacks {
		invokeScriptConfigCallback(owner, callback.key, callback.queue, callback.callback, value)
	}
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

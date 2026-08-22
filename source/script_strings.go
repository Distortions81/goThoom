package main

import "strings"

// This file exposes small, easy-to-understand string helper functions to
// script scripts.  They wrap common operations from Go's standard library in
// a friendly form so macro authors do not need to understand the entire
// strings package.

// scriptIgnoreCase reports whether a and b are the same, ignoring letter case.
func scriptIgnoreCase(a, b string) bool { return strings.EqualFold(a, b) }

// scriptStartsWith reports whether s begins with prefix.
func scriptStartsWith(s, prefix string) bool { return strings.HasPrefix(s, prefix) }

// scriptEndsWith reports whether s ends with suffix.
func scriptEndsWith(s, suffix string) bool { return strings.HasSuffix(s, suffix) }

// scriptIncludes reports whether substr appears anywhere inside s.
func scriptIncludes(s, substr string) bool { return strings.Contains(s, substr) }

// scriptLower converts all letters in s to lower case.
func scriptLower(s string) string { return strings.ToLower(s) }

// scriptUpper converts all letters in s to upper case.
func scriptUpper(s string) string { return strings.ToUpper(s) }

// scriptTrim removes leading and trailing white space from s.
func scriptTrim(s string) string { return strings.TrimSpace(s) }

// scriptTrimStart removes prefix from the start of s if it is present.
func scriptTrimStart(s, prefix string) string { return strings.TrimPrefix(s, prefix) }

// scriptTrimEnd removes suffix from the end of s if it is present.
func scriptTrimEnd(s, suffix string) string { return strings.TrimSuffix(s, suffix) }

// scriptWords splits s around spaces and returns the separate words.
func scriptWords(s string) []string { return strings.Fields(s) }

// scriptJoin joins the text elements in parts inserting sep between each one.
func scriptJoin(parts []string, sep string) string { return strings.Join(parts, sep) }

// scriptReplace returns s with every instance of old replaced by new.
func scriptReplace(s, old, new string) string { return strings.ReplaceAll(s, old, new) }

// scriptSplit breaks s into pieces separated by sep.
func scriptSplit(s, sep string) []string { return strings.Split(s, sep) }

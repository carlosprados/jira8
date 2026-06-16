package app

import "strings"

// MatchByName returns a pointer to the first item whose name (extracted via
// nameOf) matches target case-insensitively, or nil if none match. It backs the
// "resolve a transition / link type by name" lookups shared by the CLI and MCP.
func MatchByName[T any](items []T, target string, nameOf func(T) string) *T {
	for i := range items {
		if strings.EqualFold(nameOf(items[i]), target) {
			return &items[i]
		}
	}
	return nil
}

// Names maps items to their names via nameOf, preserving order. Used to build
// the "available: a, b, c" hint when a name lookup fails.
func Names[T any](items []T, nameOf func(T) string) []string {
	out := make([]string, len(items))
	for i := range items {
		out[i] = nameOf(items[i])
	}
	return out
}

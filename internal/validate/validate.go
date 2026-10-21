package validate

import (
	"sort"
	"strings"
)

// Errors maps a field path to a message the UI shows next to that field.
type Errors map[string]string

func (e Errors) Add(field, msg string) {
	if _, exists := e[field]; !exists {
		e[field] = msg
	}
}

func (e Errors) Err() error {
	if len(e) == 0 {
		return nil
	}
	return e
}

func (e Errors) Error() string {
	keys := make([]string, 0, len(e))
	for k := range e {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+": "+e[k])
	}
	return "invalid input: " + strings.Join(parts, "; ")
}

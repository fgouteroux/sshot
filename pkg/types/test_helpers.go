package types

import "fmt"

// boolPtr returns a pointer to a bool value
// This is useful for tests that need to set StrictHostKeyCheck
func BoolPtr(b bool) *bool {
	return &b
}

// compareBoolPtr compares two bool pointers for equality
// Returns true if both are nil or both point to the same value
func CompareBoolPtr(a, b *bool) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// formatBoolPtr formats a bool pointer for display in test output
func FormatBoolPtr(b *bool) string {
	if b == nil {
		return "nil"
	}
	return fmt.Sprintf("%v", *b)
}

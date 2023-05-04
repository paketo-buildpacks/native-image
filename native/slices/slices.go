package slices

// Those 2 functions were copied from https://cs.opensource.google/go/x/exp/+/master:slices/slices.go ,
// an experimental collection of utility methods that could be promoted to stdlib in the future

// Index returns the index of the first occurrence of v in s,
// or -1 if not present.
func Index[E comparable](s []E, v E) int {
	for i := range s {
		if v == s[i] {
			return i
		}
	}
	return -1
}

// Contains reports whether v is present in s.
func Contains[E comparable](s []E, v E) bool {
	return Index(s, v) >= 0
}

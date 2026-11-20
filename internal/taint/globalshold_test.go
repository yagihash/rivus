package taint

import "testing"

// A package variable that the rules say holds nothing (os.Stderr) does not tie the code together (ADR-056).
// Inside the function that writes, the variable is like any other; what leaves the function through it is not followed.
func TestGlobalHoldsNothing(t *testing.T) { checkCase(t, "globalshold") }

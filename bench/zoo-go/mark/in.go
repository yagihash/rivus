package mark

// In is the source that is a parameter type (rules.yaml: param_type). NewIn is the way a case
// makes one; the handler under test takes it. The type is the same in both builds.
type In struct{ v string }

// Value is the text of the In.
func (i In) Value() string { return i.v }

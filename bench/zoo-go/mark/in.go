package mark

// In is the source that is a parameter type (rules.yaml: param_type). NewIn is the way a case
// makes one; the handler under test takes it. The type is the same in both builds.
type In struct{ v string }

// Value is the text of the In. (On the pointer: mark is a package of the project, and the receiver of a method on In would be
// a parameter of the type, that is, a source of its own.)
func (i *In) Value() string { return i.v }

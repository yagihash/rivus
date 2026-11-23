package gin

import "zoo/mark"

// This file is not generated (the generator never touches a file that has another name than the package's own): it has what the
// zoo needs and gin does not have.

// NewContext makes the Context of a handler that a client sent the marker of tag to. (Not in gin: the cells need a way to make
// one; the real way is a router that serves a request.) The getters of the generated Context return the marker `v`.
func NewContext(tag string) *Context { return &Context{v: mark.Src(tag)} }

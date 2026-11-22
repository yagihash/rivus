// Package bind is a cell of the package gin.
//
// @cell(F/pkg/gin/bind)
// @shape: a handler of gin binds the request into a struct (ShouldBindJSON); a field of the struct reaches a sink
package bind

import (
	"github.com/gin-gonic/gin"
	"zoo/mark"
)

func init() {
	mark.Case("F/pkg/gin/bind/vuln", vuln)
	mark.Case("F/pkg/gin/bind/fixed", fixed)
	mark.Case("F/pkg/gin/bind/decoy", decoy)
}

type input struct{ Name string }

func vulnHandler(c *gin.Context) { // @source(pkg-gin-bind-vuln)
	var in input
	_ = c.ShouldBindJSON(&in)
	mark.SinkSQL(in.Name) // @sink(pkg-gin-bind-vuln)
}

func vuln() { // @vuln
	vulnHandler(gin.NewContext("pkg-gin-bind-vuln"))
}

func fixedHandler(c *gin.Context) {
	var in input
	_ = c.ShouldBindJSON(&in)
	mark.SinkSQL(mark.CleanSQL(in.Name)) // @sink(none)
}

func fixed() { // @fixed
	fixedHandler(gin.NewContext("pkg-gin-bind-fixed"))
}

func decoyHandler(c *gin.Context) { // @source(pkg-gin-bind-decoy)
	var in input
	_ = c.ShouldBindJSON(&in)
	mark.SinkSQL("constant") // @sink(none) @nosink(pkg-gin-bind-decoy)
	mark.SinkCmd(in.Name)    // @sink(pkg-gin-bind-decoy)
}

func decoy() { // @decoy
	decoyHandler(gin.NewContext("pkg-gin-bind-decoy"))
}

// Package query is a cell of the package gin.
//
// @cell(F/pkg/gin/query)
// @shape: a handler of gin takes the *gin.Context (the source) and reads a query parameter and a header; the text reaches a sink
package query

import (
	"github.com/gin-gonic/gin"
	"zoo/mark"
)

func init() {
	mark.Case("F/pkg/gin/query/vuln", vuln)
	mark.Case("F/pkg/gin/query/fixed", fixed)
	mark.Case("F/pkg/gin/query/decoy", decoy)
}

func vulnHandler(c *gin.Context) { // @source(pkg-gin-query-vuln)
	mark.SinkCmd(c.Query("q"))        // @sink(pkg-gin-query-vuln)
	mark.SinkSQL(c.GetHeader("X-Id")) // @sink(pkg-gin-query-vuln)
	mark.SinkSQL(c.Param("id"))       // @sink(pkg-gin-query-vuln)
	mark.SinkHTML(c.PostForm("name")) // @sink(pkg-gin-query-vuln)
	v, _ := c.Cookie("session")
	mark.SinkPath("/srv", v) // @sink(pkg-gin-query-vuln)
}

func vuln() { // @vuln
	vulnHandler(gin.NewContext("pkg-gin-query-vuln"))
}

func fixedHandler(c *gin.Context) {
	mark.SinkCmd(mark.CleanCmd(c.Query("q"))) // @sink(none)
}

func fixed() { // @fixed
	fixedHandler(gin.NewContext("pkg-gin-query-fixed"))
}

func decoyHandler(c *gin.Context) { // @source(pkg-gin-query-decoy)
	t := c.Query("q")
	mark.SinkCmd("constant") // @sink(none) @nosink(pkg-gin-query-decoy)
	mark.SinkSQL(t)          // @sink(pkg-gin-query-decoy)
}

func decoy() { // @decoy
	decoyHandler(gin.NewContext("pkg-gin-query-decoy"))
}

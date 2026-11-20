// Package api has the handlers of the code generator.
package api

import (
	"strings"

	"curated/gva/model"
	"curated/gva/service"
	"curated/web"
)

// AutoCodeAPI is the API of the code generator.
type AutoCodeAPI struct{}

var autoCodeService = service.AutoCodeService{}

// AutoPlug makes a plugin from the template: the body is bound to the request
// struct, the snake name is made of the plugin name, and the struct goes to the
// service by value.
func (a *AutoCodeAPI) AutoPlug(c *web.Context) {
	var req model.AutoPlugReq
	err := c.Bind(&req) // @source(gva_plug)
	if err != nil {
		c.JSON(400, err.Error())
		return
	}
	req.Snake = strings.ToLower(req.PlugName)
	req.NeedModel = req.HasRequest || req.HasResponse
	if err = autoCodeService.CreatePlug(req); err != nil {
		c.JSON(500, "failed")
		return
	}
	c.JSON(200, "ok")
}

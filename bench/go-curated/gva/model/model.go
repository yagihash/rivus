// Package model has the request of the plugin template, as in gin-vue-admin.
package model

import "strings"

// AutoPlugReq is the body of the request that makes a plugin from the template.
type AutoPlugReq struct {
	PlugName    string `json:"plugName"`
	Snake       string `json:"snake"`
	HasRequest  bool   `json:"hasRequest"`
	HasResponse bool   `json:"hasResponse"`
	NeedModel   bool   `json:"needModel"`
}

// CheckList normalises the names (it does not check the path).
func (r *AutoPlugReq) CheckList() {
	r.PlugName = strings.TrimSpace(r.PlugName)
}

// Package service writes the files of the generated code.
package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"curated/gva/model"
)

// config is the global configuration (the paths of the generator).
var config = struct{ Root, Server, SPlug string }{"/srv", "server", "plugin/%s"}

const plugPath = "resource/plug/"

// AutoCodeService is the service of the code generator.
type AutoCodeService struct{}

// getAllTplFile lists the template files under a directory (constants).
func (s *AutoCodeService) getAllTplFile(dir string) []string {
	return []string{dir + "api/a.go.tpl", dir + "router/r.go.tpl"}
}

// CreatePlug makes the plugin from the template: for every template file a
// directory and a file under the plugin directory, named by the plugin name.
func (s *AutoCodeService) CreatePlug(plug model.AutoPlugReq) error {
	plug.CheckList()
	for _, tpl := range s.getAllTplFile(plugPath) {
		pathArr := strings.SplitAfter(tpl, "/")
		if !strings.Contains(pathArr[2], "tpl") {
			dirPath := filepath.Join(config.Root, config.Server, fmt.Sprintf(config.SPlug, plug.Snake+"/"+pathArr[2]))
			os.MkdirAll(dirPath, 0755) // @sink(gva_plug)
		}
		file := filepath.Join(config.Root, config.Server, fmt.Sprintf(config.SPlug, plug.Snake+"/"+tpl[len(plugPath):len(tpl)-4]))
		f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE, 0666) // @sink(gva_plug)
		if err != nil {
			return err
		}
		defer f.Close()
	}
	return nil
}

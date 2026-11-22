// Package gin is a skeleton of github.com/gin-gonic/gin v1.12.0 for the zoo (docs/DESIGN-package-models.md, 6.8): the public
// signatures of the request side of Context that the built-in model (internal/model/builtin/go-gin.yaml) has rows for, and
// bodies that only carry the marker of the case, so that the cells run. It is not gin: the analysis does not read these bodies
// where the model has a row, and the real gin is never fetched or run.
package gin

import (
	"io/fs"
	"mime/multipart"
	"net/http"
	"reflect"

	"zoo/mark"
)

// Context is the request and the response of a handler. v is the text that every getter of the skeleton returns.
type Context struct {
	Request *http.Request
	Params  Params
	v       string
}

// NewContext makes the Context of a handler that a client sent the marker of tag to. (Not in gin: the cells need a way to make
// one; the real way is a router that serves a request.)
func NewContext(tag string) *Context { return &Context{v: mark.Src(tag)} }

// Param is one segment of the path that a route captured, and Params all of them.
type Param struct{ Key, Value string }

// Params is the segments of the path that a route captured.
type Params []Param

// Get returns the value of the segment named name.
func (ps Params) Get(name string) (string, bool) {
	for _, p := range ps {
		if p.Key == name {
			return p.Value, true
		}
	}
	return "", false
}

// ByName is Get without the flag.
func (ps Params) ByName(name string) string { v, _ := ps.Get(name); return v }

func (c *Context) Query(key string) string             { return c.v }
func (c *Context) DefaultQuery(key, def string) string { return c.v }
func (c *Context) GetQuery(key string) (string, bool)  { return c.v, true }
func (c *Context) QueryArray(key string) []string      { return []string{c.v} }
func (c *Context) GetQueryArray(key string) ([]string, bool) {
	return []string{c.v}, true
}
func (c *Context) QueryMap(key string) map[string]string { return map[string]string{key: c.v} }
func (c *Context) GetQueryMap(key string) (map[string]string, bool) {
	return map[string]string{key: c.v}, true
}
func (c *Context) Param(key string) string                { return c.v }
func (c *Context) PostForm(key string) string             { return c.v }
func (c *Context) DefaultPostForm(key, def string) string { return c.v }
func (c *Context) GetPostForm(key string) (string, bool)  { return c.v, true }
func (c *Context) PostFormArray(key string) []string      { return []string{c.v} }
func (c *Context) GetPostFormArray(key string) ([]string, bool) {
	return []string{c.v}, true
}
func (c *Context) PostFormMap(key string) map[string]string { return map[string]string{key: c.v} }
func (c *Context) GetPostFormMap(key string) (map[string]string, bool) {
	return map[string]string{key: c.v}, true
}
func (c *Context) FormFile(name string) (*multipart.FileHeader, error) {
	return &multipart.FileHeader{Filename: c.v}, nil
}
func (c *Context) MultipartForm() (*multipart.Form, error) {
	return &multipart.Form{Value: map[string][]string{"v": {c.v}}}, nil
}
func (c *Context) GetHeader(key string) string        { return c.v }
func (c *Context) Cookie(name string) (string, error) { return c.v, nil }
func (c *Context) GetRawData() ([]byte, error)        { return []byte(c.v), nil }
func (c *Context) ClientIP() string                   { return c.v }
func (c *Context) FullPath() string                   { return c.v }
func (c *Context) ContentType() string                { return c.v }

// fill sets the string fields of the struct that obj points to to the text of the request.
func (c *Context) fill(obj any) error {
	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Pointer || v.Elem().Kind() != reflect.Struct {
		return nil
	}
	e := v.Elem()
	for i := 0; i < e.NumField(); i++ {
		if f := e.Field(i); f.Kind() == reflect.String && f.CanSet() {
			f.SetString(c.v)
		}
	}
	return nil
}

// Binding and BindingBody stand for the binders that the real gin has in package binding.
type Binding interface{ Name() string }
type BindingBody interface{ Name() string }

func (c *Context) Bind(obj any) error                              { return c.fill(obj) }
func (c *Context) BindJSON(obj any) error                          { return c.fill(obj) }
func (c *Context) BindXML(obj any) error                           { return c.fill(obj) }
func (c *Context) BindQuery(obj any) error                         { return c.fill(obj) }
func (c *Context) BindYAML(obj any) error                          { return c.fill(obj) }
func (c *Context) BindTOML(obj any) error                          { return c.fill(obj) }
func (c *Context) BindPlain(obj any) error                         { return c.fill(obj) }
func (c *Context) BindHeader(obj any) error                        { return c.fill(obj) }
func (c *Context) BindUri(obj any) error                           { return c.fill(obj) }
func (c *Context) MustBindWith(obj any, b Binding) error           { return c.fill(obj) }
func (c *Context) ShouldBind(obj any) error                        { return c.fill(obj) }
func (c *Context) ShouldBindJSON(obj any) error                    { return c.fill(obj) }
func (c *Context) ShouldBindXML(obj any) error                     { return c.fill(obj) }
func (c *Context) ShouldBindQuery(obj any) error                   { return c.fill(obj) }
func (c *Context) ShouldBindYAML(obj any) error                    { return c.fill(obj) }
func (c *Context) ShouldBindTOML(obj any) error                    { return c.fill(obj) }
func (c *Context) ShouldBindPlain(obj any) error                   { return c.fill(obj) }
func (c *Context) ShouldBindHeader(obj any) error                  { return c.fill(obj) }
func (c *Context) ShouldBindUri(obj any) error                     { return c.fill(obj) }
func (c *Context) ShouldBindWith(obj any, b Binding) error         { return c.fill(obj) }
func (c *Context) ShouldBindBodyWith(obj any, b BindingBody) error { return c.fill(obj) }
func (c *Context) ShouldBindBodyWithJSON(obj any) error            { return c.fill(obj) }
func (c *Context) ShouldBindBodyWithXML(obj any) error             { return c.fill(obj) }
func (c *Context) ShouldBindBodyWithYAML(obj any) error            { return c.fill(obj) }
func (c *Context) ShouldBindBodyWithTOML(obj any) error            { return c.fill(obj) }
func (c *Context) ShouldBindBodyWithPlain(obj any) error           { return c.fill(obj) }

// The operations on the response: they record what they got and do nothing.
func (c *Context) Redirect(code int, location string)               {}
func (c *Context) File(filepath string)                             {}
func (c *Context) FileFromFS(filepath string, fsys http.FileSystem) {}
func (c *Context) FileAttachment(filepath, filename string)         {}
func (c *Context) SaveUploadedFile(file *multipart.FileHeader, dst string, perm ...fs.FileMode) error {
	return nil
}
func (c *Context) String(code int, format string, values ...any)  {}
func (c *Context) Data(code int, contentType string, data []byte) {}

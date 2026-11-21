//go:build zooexec

// Package pure calls the functions that the built-in model of rivus says are pure (no taint flows
// through them) with a marker, and writes what comes out. cmd/rivus TestZooPureGo runs it with the
// two variants of the marker (bench/zoo/spec.json): a result that is not the same in both depends on
// the marker, so the function carries it, and a pure row for it is wrong (R-03).
//
// Only the rows whose arguments are strings or bytes, and that are safe to call (no file is made, no
// process runs, no network, no output), are here. The answer is from the run, not from the model.
package pure

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"zoo/mark"
)

// Part is one result of a call: the value, or the text of the error.
type Part struct {
	Symbol string `json:"symbol"` // the symbol of the model
	Part   string `json:"part"`   // "value" or "error"
	Text   string `json:"text"`
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

type row struct {
	symbol string
	call   func(s string) []Part
}

func v(symbol string, x any) Part     { return Part{symbol, "value", fmt.Sprintf("%v", x)} }
func e(symbol string, err error) Part { return Part{symbol, "error", errText(err)} }

var rows = []row{
	{"strings.HasPrefix", func(s string) []Part { return []Part{v("strings.HasPrefix", strings.HasPrefix(s, "Z"))} }},
	{"strings.HasSuffix", func(s string) []Part { return []Part{v("strings.HasSuffix", strings.HasSuffix(s, "x"))} }},
	{"strings.Contains", func(s string) []Part { return []Part{v("strings.Contains", strings.Contains(s, "SH"))} }},
	{"strings.EqualFold", func(s string) []Part { return []Part{v("strings.EqualFold", strings.EqualFold(s, "z"))} }},
	{"strings.Index", func(s string) []Part { return []Part{v("strings.Index", strings.Index(s, ".SQL"))} }},
	{"strings.LastIndex", func(s string) []Part { return []Part{v("strings.LastIndex", strings.LastIndex(s, ".PATH"))} }},
	{"strings.Count", func(s string) []Part { return []Part{v("strings.Count", strings.Count(s, "."))} }},
	{"strings.Compare", func(s string) []Part { return []Part{v("strings.Compare", strings.Compare(s, "Z"))} }},
	{"strings.ContainsAny", func(s string) []Part { return []Part{v("strings.ContainsAny", strings.ContainsAny(s, "Z"))} }},
	{"strings.IndexByte", func(s string) []Part { return []Part{v("strings.IndexByte", strings.IndexByte(s, '.'))} }},
	{"bytes.Equal", func(s string) []Part { return []Part{v("bytes.Equal", bytes.Equal([]byte(s), []byte("Z")))} }},
	{"bytes.Index", func(s string) []Part { return []Part{v("bytes.Index", bytes.Index([]byte(s), []byte(".SH")))} }},
	{"bytes.HasPrefix", func(s string) []Part { return []Part{v("bytes.HasPrefix", bytes.HasPrefix([]byte(s), []byte("Z")))} }},
	{"unicode/utf8.ValidString", func(s string) []Part { return []Part{v("unicode/utf8.ValidString", utf8.ValidString(s))} }},
	{"unicode/utf8.RuneCountInString", func(s string) []Part {
		return []Part{v("unicode/utf8.RuneCountInString", utf8.RuneCountInString(s))}
	}},
	{"unicode/utf8.Valid", func(s string) []Part { return []Part{v("unicode/utf8.Valid", utf8.Valid([]byte(s)))} }},
	{"encoding/json.Valid", func(s string) []Part { return []Part{v("encoding/json.Valid", json.Valid([]byte(s)))} }},
	{"path/filepath.IsAbs", func(s string) []Part { return []Part{v("path/filepath.IsAbs", filepath.IsAbs(s))} }},
	{"net/http.DetectContentType", func(s string) []Part {
		return []Part{v("net/http.DetectContentType", http.DetectContentType([]byte(s)))}
	}},
	{"strconv.Atoi", func(s string) []Part {
		n, err := strconv.Atoi(s)
		return []Part{v("strconv.Atoi", n), e("strconv.Atoi", err)}
	}},
	{"strconv.ParseBool", func(s string) []Part {
		b, err := strconv.ParseBool(s)
		return []Part{v("strconv.ParseBool", b), e("strconv.ParseBool", err)}
	}},
	{"strconv.ParseFloat", func(s string) []Part {
		f, err := strconv.ParseFloat(s, 64)
		return []Part{v("strconv.ParseFloat", f), e("strconv.ParseFloat", err)}
	}},
	{"strconv.ParseUint", func(s string) []Part {
		n, err := strconv.ParseUint(s, 10, 64)
		return []Part{v("strconv.ParseUint", n), e("strconv.ParseUint", err)}
	}},
	{"time.Parse", func(s string) []Part {
		t, err := time.Parse("2006-01-02", s)
		return []Part{v("time.Parse", t.IsZero()), e("time.Parse", err)}
	}},
	{"time.ParseInLocation", func(s string) []Part {
		t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
		return []Part{v("time.ParseInLocation", t.IsZero()), e("time.ParseInLocation", err)}
	}},
	{"time.ParseDuration", func(s string) []Part {
		d, err := time.ParseDuration(s)
		return []Part{v("time.ParseDuration", int64(d)), e("time.ParseDuration", err)}
	}},
	{"(time.Time).Format", func(s string) []Part {
		return []Part{v("(time.Time).Format", time.Unix(0, 0).UTC().Format(s))}
	}},
	{"os/exec.LookPath", func(s string) []Part {
		p, err := exec.LookPath(s)
		return []Part{v("os/exec.LookPath", p), e("os/exec.LookPath", err)}
	}},
	{"os.Stat", func(s string) []Part {
		fi, err := os.Stat(s)
		name := ""
		if fi != nil {
			name = fi.Name()
		}
		return []Part{v("os.Stat", name), e("os.Stat", err)}
	}},
	{"regexp.Compile", func(s string) []Part {
		re, err := regexp.Compile(regexp.QuoteMeta(s))
		str := ""
		if re != nil {
			str = re.String()
		}
		return []Part{v("regexp.Compile", str), e("regexp.Compile", err)}
	}},
	{"(*regexp.Regexp).MatchString", func(s string) []Part {
		return []Part{v("(*regexp.Regexp).MatchString", regexp.MustCompile(`SH`).MatchString(s))}
	}},
	{"encoding/hex.Encode", func(s string) []Part {
		dst := make([]byte, hex.EncodedLen(len(s)))
		hex.Encode(dst, []byte(s))
		return []Part{v("encoding/hex.Encode", string(dst))}
	}},
	{"encoding/hex.AppendEncode", func(s string) []Part {
		return []Part{v("encoding/hex.AppendEncode", string(hex.AppendEncode(nil, []byte(s))))}
	}},
	{"net/url.UserPassword", func(s string) []Part {
		return []Part{v("net/url.UserPassword", url.UserPassword("u", s).String())}
	}},
	{"(*net/url.URL).RequestURI", func(s string) []Part {
		u := &url.URL{Path: "/" + s}
		return []Part{v("(*net/url.URL).RequestURI", u.RequestURI())}
	}},
	{"(*net/http.Cookie).String", func(s string) []Part {
		c := &http.Cookie{Name: "a", Value: s}
		return []Part{v("(*net/http.Cookie).String", c.String())}
	}},
	{"net/netip.ParseAddr", func(s string) []Part {
		a, err := netip.ParseAddr(s)
		return []Part{v("net/netip.ParseAddr", a.String()), e("net/netip.ParseAddr", err)}
	}},
	{"(*encoding/json.Decoder).Token", func(s string) []Part {
		q, _ := json.Marshal(s)
		tok, err := json.NewDecoder(strings.NewReader(string(q))).Token()
		return []Part{v("(*encoding/json.Decoder).Token", tok), e("(*encoding/json.Decoder).Token", err)}
	}},
	// The recipes of receivers (R-03): a value of the type is made with the marker in it, and the methods that the model
	// says are pure are called. The receiver is the way in that a call with the marker in an argument does not cover.
	{"(*log/slog.Level).UnmarshalText", func(s string) []Part {
		var l slog.Level
		err := l.UnmarshalText([]byte(s))
		return []Part{v("(*log/slog.Level).UnmarshalText", l), e("(*log/slog.Level).UnmarshalText", err)}
	}},
	{"(*regexp.Regexp).Match", func(s string) []Part {
		return []Part{v("(*regexp.Regexp).Match", regexp.MustCompile(`SH`).Match([]byte(s)))}
	}},
	{"(*net/url.URL).Port", func(s string) []Part {
		u, err := url.Parse("http://host:8080/" + s)
		if err != nil {
			return []Part{e("(*net/url.URL).Port", err)}
		}
		return []Part{v("(*net/url.URL).Port", u.Port())}
	}},
	{"(*os/exec.Cmd).Run", func(s string) []Part {
		// the marker is the name of the command: a command that is not found has its name in the error
		err := exec.Command(s).Run()
		return []Part{e("(*os/exec.Cmd).Run", err)}
	}},
}

// Run calls every row with the marker of the variant that runs.
func Run() []Part {
	var out []Part
	for _, r := range rows {
		out = append(out, r.call(mark.Src("pure-"+strconv.Itoa(len(out))))...)
	}
	return out
}

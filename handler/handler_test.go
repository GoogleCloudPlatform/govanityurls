// Copyright 2017 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handler

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
)

// TestHandler tests basic handler functionality.
func TestHandler(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		path   string

		goImport string
		goSource string
	}{
		{
			name: "explicit display",
			config: Config{
				Host: "example.com",
				Paths: map[string]ConfigPath{
					"/portmidi": {
						Repo:    "https://github.com/rakyll/portmidi",
						Display: "https://github.com/rakyll/portmidi _ _",
					},
				},
			},
			path:     "/portmidi",
			goImport: "example.com/portmidi git https://github.com/rakyll/portmidi",
			goSource: "example.com/portmidi https://github.com/rakyll/portmidi _ _",
		},
		{
			name: "display GitHub inference",
			config: Config{
				Host: "example.com",
				Paths: map[string]ConfigPath{
					"/portmidi": {
						Repo: "https://github.com/rakyll/portmidi",
					},
				},
			},
			path:     "/portmidi",
			goImport: "example.com/portmidi git https://github.com/rakyll/portmidi",
			goSource: "example.com/portmidi https://github.com/rakyll/portmidi https://github.com/rakyll/portmidi/tree/master{/dir} https://github.com/rakyll/portmidi/blob/master{/dir}/{file}#L{line}",
		},
		{
			name: "Bitbucket Mercurial",
			config: Config{
				Host: "example.com",
				Paths: map[string]ConfigPath{
					"/gopdf": {
						Repo: "https://bitbucket.org/zombiezen/gopdf",
						VCS:  "hg",
					},
				},
			},
			path:     "/gopdf",
			goImport: "example.com/gopdf hg https://bitbucket.org/zombiezen/gopdf",
			goSource: "example.com/gopdf https://bitbucket.org/zombiezen/gopdf https://bitbucket.org/zombiezen/gopdf/src/default{/dir} https://bitbucket.org/zombiezen/gopdf/src/default{/dir}/{file}#{file}-{line}",
		},
		{
			name: "Bitbucket Git",
			config: Config{
				Host: "example.com",
				Paths: map[string]ConfigPath{
					"/mygit": {
						Repo: "https://bitbucket.org/zombiezen/mygit",
						VCS:  "git",
					},
				},
			},
			path:     "/mygit",
			goImport: "example.com/mygit git https://bitbucket.org/zombiezen/mygit",
			goSource: "example.com/mygit https://bitbucket.org/zombiezen/mygit https://bitbucket.org/zombiezen/mygit/src/default{/dir} https://bitbucket.org/zombiezen/mygit/src/default{/dir}/{file}#{file}-{line}",
		},
		{
			name: "subpath",
			config: Config{
				Host: "example.com",
				Paths: map[string]ConfigPath{
					"/portmidi": {
						Repo:    "https://github.com/rakyll/portmidi",
						Display: "https://github.com/rakyll/portmidi _ _",
					},
				},
			},
			path:     "/portmidi/foo",
			goImport: "example.com/portmidi git https://github.com/rakyll/portmidi",
			goSource: "example.com/portmidi https://github.com/rakyll/portmidi _ _",
		},
		{
			name: "subpath with trailing config slash",
			config: Config{
				Host: "example.com",
				Paths: map[string]ConfigPath{
					"/portmidi/": {
						Repo:    "https://github.com/rakyll/portmidi",
						Display: "https://github.com/rakyll/portmidi _ _",
					},
				},
			},
			path:     "/portmidi/foo",
			goImport: "example.com/portmidi git https://github.com/rakyll/portmidi",
			goSource: "example.com/portmidi https://github.com/rakyll/portmidi _ _",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h, err := New(test.config)
			if err != nil {
				t.Errorf("New: %v", err)
				return
			}
			s := httptest.NewServer(h)
			resp, err := http.Get(s.URL + test.path)
			if err != nil {
				s.Close()
				t.Errorf("http.Get: %v", err)
				return
			}
			data, err := ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			s.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("status code = %s; want 200 OK", resp.Status)
			}
			if err != nil {
				t.Errorf("ioutil.ReadAll: %v", err)
				return
			}
			if got := findMeta(data, "go-import"); got != test.goImport {
				t.Errorf("meta go-import = %q; want %q", got, test.goImport)
			}
			if got := findMeta(data, "go-source"); got != test.goSource {
				t.Errorf("meta go-source = %q; want %q", got, test.goSource)
			}
		})
	}
}

// TestBadConfigs tests error handling for invalid configuration.
func TestBadConfigs(t *testing.T) {
	negativeCacheAge := int64(-1)
	badConfigs := map[string]Config{
		"missing vcs": {
			Paths: map[string]ConfigPath{
				"/missingvcs": {
					Repo: "https://bitbucket.org/zombiezen/gopdf",
				},
			},
		},
		"unknown vcs": {
			Paths: map[string]ConfigPath{
				"/unknownvcs": {
					Repo: "https://bitbucket.org/zombiezen/gopdf",
					VCS:  "xyzzy",
				},
			},
		},
		"bad cache_max_age": {
			Paths: map[string]ConfigPath{
				"/portmidi": {
					Repo: "https://github.com/rakyll/portmidi",
				},
			},
			CacheAge: &negativeCacheAge,
		},
	}
	for name, config := range badConfigs {
		t.Run(name, func(t *testing.T) {
			_, err := New(config)
			if err == nil {
				t.Errorf("expected config to produce an error, but did not:\n%#v", config)
			}
		})
	}
}

func findMeta(data []byte, name string) string {
	var sep []byte
	sep = append(sep, `<meta name="`...)
	sep = append(sep, name...)
	sep = append(sep, `" content="`...)
	i := bytes.Index(data, sep)
	if i == -1 {
		return ""
	}
	content := data[i+len(sep):]
	j := bytes.IndexByte(content, '"')
	if j == -1 {
		return ""
	}
	return string(content[:j])
}

// TestPathConfigSetFind tests configpath search logic.
func TestPathConfigSetFind(t *testing.T) {
	tests := []struct {
		paths   []string
		query   string
		want    string
		subpath string
	}{
		{
			paths: []string{"/portmidi"},
			query: "/portmidi",
			want:  "/portmidi",
		},
		{
			paths: []string{"/portmidi"},
			query: "/portmidi/",
			want:  "/portmidi",
		},
		{
			paths: []string{"/portmidi"},
			query: "/foo",
			want:  "",
		},
		{
			paths: []string{"/portmidi"},
			query: "/zzz",
			want:  "",
		},
		{
			paths: []string{"/abc", "/portmidi", "/xyz"},
			query: "/portmidi",
			want:  "/portmidi",
		},
		{
			paths:   []string{"/abc", "/portmidi", "/xyz"},
			query:   "/portmidi/foo",
			want:    "/portmidi",
			subpath: "foo",
		},
		{
			paths:   []string{"/example/helloworld", "/", "/y", "/foo"},
			query:   "/x",
			want:    "/",
			subpath: "x",
		},
		{
			paths:   []string{"/example/helloworld", "/", "/y", "/foo"},
			query:   "/",
			want:    "/",
			subpath: "",
		},
		{
			paths:   []string{"/example/helloworld", "/", "/y", "/foo"},
			query:   "/example",
			want:    "/",
			subpath: "example",
		},
		{
			paths:   []string{"/example/helloworld", "/", "/y", "/foo"},
			query:   "/example/foo",
			want:    "/",
			subpath: "example/foo",
		},
		{
			paths: []string{"/example/helloworld", "/", "/y", "/foo"},
			query: "/y",
			want:  "/y",
		},
		{
			paths:   []string{"/example/helloworld", "/", "/y", "/foo"},
			query:   "/x/y/",
			want:    "/",
			subpath: "x/y/",
		},
		{
			paths: []string{"/example/helloworld", "/y", "/foo"},
			query: "/x",
			want:  "",
		},
	}
	emptyToNil := func(s string) string {
		if s == "" {
			return "<nil>"
		}
		return s
	}
	for _, test := range tests {
		t.Run(test.query, func(t *testing.T) {
			pset := make(pathConfigSet, len(test.paths))
			for i := range test.paths {
				pset[i].path = test.paths[i]
			}
			sort.Sort(pset)
			pc, subpath := pset.find(test.query)
			var got string
			if pc != nil {
				got = pc.path
			}
			if got != test.want || subpath != test.subpath {
				t.Errorf("pathConfigSet(%v).find(%q) = %v, %v; want %v, %v",
					test.paths, test.query, emptyToNil(got), subpath, emptyToNil(test.want), test.subpath)
			}
		})
	}
}

// TestCacheHeader tests generation of the Cache-Control header.
func TestCacheHeader(t *testing.T) {
	zeroAge := int64(0)
	longAge := int64(60)

	tests := []struct {
		name         string
		config       Config
		cacheControl string
	}{
		{
			name: "default",
			config: Config{
				Paths: map[string]ConfigPath{
					"/portmidi": {
						Repo: "https://github.com/rakyll/portmidi",
					},
				},
			},
			cacheControl: "public, max-age=86400",
		},
		{
			name: "specify time",
			config: Config{
				Paths: map[string]ConfigPath{
					"/portmidi": {
						Repo: "https://github.com/rakyll/portmidi",
					},
				},
				CacheAge: &longAge,
			},
			cacheControl: "public, max-age=60",
		},
		{
			name: "zero config_max_age",
			config: Config{
				Paths: map[string]ConfigPath{
					"/portmidi": {
						Repo: "https://github.com/rakyll/portmidi",
					},
				},
				CacheAge: &zeroAge,
			},
			cacheControl: "public, max-age=0",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h, err := New(test.config)
			if err != nil {
				t.Errorf("newHandler: %v", err)
				return
			}
			s := httptest.NewServer(h)
			resp, err := http.Get(s.URL + "/portmidi")
			if err != nil {
				t.Errorf("http.Get: %v", err)
				return
			}
			resp.Body.Close()
			got := resp.Header.Get("Cache-Control")
			if got != test.cacheControl {
				t.Errorf("Cache-Control header = %q; want %q", got, test.cacheControl)
			}
		})
	}
}

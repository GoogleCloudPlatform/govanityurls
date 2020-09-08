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

// Package handler implements the http handler for govanityurls.
package handler

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"
)

type handler struct {
	hostName     string
	cacheControl string
	paths        pathConfigSet
}

type pathConfig struct {
	path    string
	repo    string
	display string
	vcs     string
}

type ConfigPath struct {
	Repo    string `yaml:"repo,omitempty"`
	Display string `yaml:"display,omitempty"`
	VCS     string `yaml:"vcs,omitempty"`
}

type Config struct {
	Host     string                `yaml:"host,omitempty"`
	CacheAge *int64                `yaml:"cache_max_age,omitempty"`
	Paths    map[string]ConfigPath `yaml:"paths,omitempty"`
}

// New returns an http.Handler based on provided configuration. The handler will
// respond to `go get` requests and redirect to the right repository.
func New(config Config) (http.Handler, error) {
	h := &handler{hostName: config.Host}
	cacheAge := int64(86400) // 24 hours (in seconds)
	if config.CacheAge != nil {
		cacheAge = *config.CacheAge
		if cacheAge < 0 {
			return nil, errors.New("cache_max_age is negative")
		}
	}
	h.cacheControl = fmt.Sprintf("public, max-age=%d", cacheAge)
	for path, e := range config.Paths {
		pc := pathConfig{
			path:    strings.TrimSuffix(path, "/"),
			repo:    e.Repo,
			display: e.Display,
			vcs:     e.VCS,
		}
		switch {
		case e.Display != "":
			// Already filled in.
		case strings.HasPrefix(e.Repo, "https://github.com/"):
			pc.display = fmt.Sprintf("%v %v/tree/master{/dir} %v/blob/master{/dir}/{file}#L{line}", e.Repo, e.Repo, e.Repo)
		case strings.HasPrefix(e.Repo, "https://bitbucket.org"):
			pc.display = fmt.Sprintf("%v %v/src/default{/dir} %v/src/default{/dir}/{file}#{file}-{line}", e.Repo, e.Repo, e.Repo)
		}
		switch {
		case e.VCS != "":
			// Already filled in.
			if e.VCS != "bzr" && e.VCS != "git" && e.VCS != "hg" && e.VCS != "svn" {
				return nil, fmt.Errorf("configuration for %v: unknown VCS %s", path, e.VCS)
			}
		case strings.HasPrefix(e.Repo, "https://github.com/"):
			pc.vcs = "git"
		default:
			return nil, fmt.Errorf("configuration for %v: cannot infer VCS from %s", path, e.Repo)
		}
		h.paths = append(h.paths, pc)
	}
	sort.Sort(h.paths)
	return h, nil
}

// ParseConfig parses a slice of bytes containing yaml configuration and
// returns a Config instance.
func ParseConfig(config []byte) (Config, error) {
	var parsed Config
	if err := yaml.Unmarshal(config, &parsed); err != nil {
		return parsed, err
	}
	return parsed, nil
}

// ServeHTTP serves handles go get requests.
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	current := r.URL.Path
	// We check for the paths that don't start with / here as some middleware
	// like http.StripPrefix will strip prefix including a trailing slash.
	// e.g. http.Handle("/vanity/", http.StripPrefix("/vanity/", h))
	if !strings.HasPrefix(current, "/") {
		current = "/" + current
	}

	pc, subpath := h.paths.find(current)
	if pc == nil && current == "/" {
		h.serveIndex(w, r)
		return
	}
	if pc == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Cache-Control", h.cacheControl)
	if err := vanityTmpl.Execute(w, struct {
		Import  string
		Subpath string
		Repo    string
		Display string
		VCS     string
	}{
		Import:  h.host(r) + pc.path,
		Subpath: subpath,
		Repo:    pc.repo,
		Display: pc.display,
		VCS:     pc.vcs,
	}); err != nil {
		http.Error(w, "cannot render the page", http.StatusInternalServerError)
	}
}

func (h *handler) serveIndex(w http.ResponseWriter, r *http.Request) {
	host := h.host(r)
	handlers := make([]string, len(h.paths))
	for i, h := range h.paths {
		handlers[i] = host + h.path
	}
	if err := indexTmpl.Execute(w, struct {
		Host     string
		Handlers []string
	}{
		Host:     host,
		Handlers: handlers,
	}); err != nil {
		http.Error(w, "cannot render the page", http.StatusInternalServerError)
	}
}

func (h *handler) host(r *http.Request) string {
	host := h.hostName
	if host == "" {
		// Default to using the requested host name.
		host = r.Host
	}
	return host
}

var indexTmpl = template.Must(template.New("index").Parse(`<!DOCTYPE html>
<html>
<h1>{{.Host}}</h1>
<ul>
{{range .Handlers}}<li><a href="https://pkg.go.dev/{{.}}">{{.}}</a></li>{{end}}
</ul>
</html>
`))

var vanityTmpl = template.Must(template.New("vanity").Parse(`<!DOCTYPE html>
<html>
<head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>
<meta name="go-import" content="{{.Import}} {{.VCS}} {{.Repo}}">
<meta name="go-source" content="{{.Import}} {{.Display}}">
<meta http-equiv="refresh" content="0; url=https://pkg.go.dev/{{.Import}}/{{.Subpath}}">
</head>
<body>
Nothing to see here; <a href="https://pkg.go.dev/{{.Import}}/{{.Subpath}}">see the package on pkg.go.dev</a>.
</body>
</html>`))

type pathConfigSet []pathConfig

func (pset pathConfigSet) Len() int {
	return len(pset)
}

func (pset pathConfigSet) Less(i, j int) bool {
	return pset[i].path < pset[j].path
}

func (pset pathConfigSet) Swap(i, j int) {
	pset[i], pset[j] = pset[j], pset[i]
}

func (pset pathConfigSet) find(path string) (pc *pathConfig, subpath string) {
	// Fast path with binary search to retrieve exact matches
	// e.g. given pset ["/", "/abc", "/xyz"], path "/def" won't match.
	i := sort.Search(len(pset), func(i int) bool {
		return pset[i].path >= path
	})
	if i < len(pset) && pset[i].path == path {
		return &pset[i], ""
	}
	if i > 0 && strings.HasPrefix(path, pset[i-1].path+"/") {
		return &pset[i-1], path[len(pset[i-1].path)+1:]
	}

	// Slow path, now looking for the longest prefix/shortest subpath i.e.
	// e.g. given pset ["/", "/abc/", "/abc/def/", "/xyz"/]
	//  * query "/abc/foo" returns "/abc/" with a subpath of "foo"
	//  * query "/x" returns "/" with a subpath of "x"
	lenShortestSubpath := len(path)
	var bestMatchConfig *pathConfig

	// After binary search with the >= lexicographic comparison,
	// nothing greater than i will be a prefix of path.
	max := i
	for i := 0; i < max; i++ {
		ps := pset[i]
		if len(ps.path) >= len(path) {
			// We previously didn't find the path by search, so any
			// route with equal or greater length is NOT a match.
			continue
		}
		sSubpath := strings.TrimPrefix(path, ps.path)
		if len(sSubpath) < lenShortestSubpath {
			subpath = sSubpath
			lenShortestSubpath = len(sSubpath)
			bestMatchConfig = &pset[i]
		}
	}
	return bestMatchConfig, subpath
}

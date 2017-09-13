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

// govanityurls serves Go vanity URLs.
package main

import (
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"
)

type handler struct {
	host      string
	paths     pathConfigSet
	pathRules pathConfigRuleSet
}

type pathConfig struct {
	path    string
	repo    string
	display string
	vcs     string
}

type pathConfigRule struct {
	placeholder string
	repoSubst   string
	display     string
	vcs         string
}

type parsedPaths map[string]struct {
	Repo    string `yaml:"repo,omitempty"`
	Display string `yaml:"display,omitempty"`
	VCS     string `yaml:"vcs,omitempty"`
}
type parsedPathRules map[string]struct {
	Repo    string `yaml:"repo,omitempty"`
	Display string `yaml:"display,omitempty"`
	VCS     string `yaml:"vcs,omitempty"`
}

func findStructure(match string) (prefix, placeholder, suffix string, err error) {
	s := match
	i := strings.Index(s, "{")
	if i < 0 {
		return "", "", "", fmt.Errorf("no placeholder found in %q", match)
	}
	prefix, s = s[:i], s[i+1:]
	i = strings.Index(s, "}")
	if i < 0 {
		return "", "", "", fmt.Errorf("placeholder not terminated in %q", match)
	}
	if i == 0 {
		return "", "", "", fmt.Errorf("placeholder is empty in %q", match)
	}
	placeholder, suffix = s[:i], s[i+1:]
	if strings.ContainsAny(suffix, "{}") {
		return "", "", "", fmt.Errorf("multiple placeholders in %q and only one allowed", match)
	}
	return prefix, "{" + placeholder + "}", suffix, nil

}

func parsePathRules(pathRules parsedPathRules) (pathConfigRuleSet, error) {
	var paths pathConfigRuleSet
	for rule, e := range pathRules {
		prefix, placeholder, suffix, err := findStructure(strings.TrimSuffix(rule, "/"))
		if err != nil {
			return nil, err
		}
		if suffix != "" {
			return nil, fmt.Errorf("configuration for %v: trailing garbage %q after placeholder %q", rule, suffix, placeholder)
		}
		_, repoPlaceHolder, _, err := findStructure(strings.TrimSuffix(e.Repo, "/"))
		if err != nil {
			return nil, fmt.Errorf("configuration for %v: repo", err)
		}
		if placeholder != repoPlaceHolder {
			return nil, fmt.Errorf("configuration for %v: placeholder in rule is %q but %q in repo", rule, placeholder, repoPlaceHolder)
		}
		pc := pathConfigRule{
			placeholder: placeholder,
			repoSubst:   e.Repo,
			display:     e.Display,
			vcs:         e.VCS,
		}
		switch {
		case e.VCS != "":
			// Already filled in.
			if e.VCS != "bzr" && e.VCS != "git" && e.VCS != "hg" && e.VCS != "svn" {
				return nil, fmt.Errorf("configuration for %v: unknown VCS %s", rule, e.VCS)
			}
		case strings.HasPrefix(e.Repo, "https://github.com/"):
			pc.vcs = "git"
		default:
			return nil, fmt.Errorf("configuration for %v: cannot infer VCS from %s", rule, e.Repo)
		}
		if paths[prefix] != nil {
			return nil, fmt.Errorf("configuration for %v: duplicate prefix %s", rule, prefix)
		}
		if paths == nil {
			paths = make(pathConfigRuleSet)
		}
		paths[prefix] = &pc
	}
	for prefix := range paths {
		for value := range paths {
			if value == prefix {
				continue
			}
			if strings.HasPrefix(value, prefix) {
				return nil, fmt.Errorf("configuration for %v is already covered by %v", value, prefix)
			}
		}
	}
	return paths, nil
}

func parsePaths(pathTable parsedPaths) (pathConfigSet, error) {
	var paths pathConfigSet
	for path, e := range pathTable {
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
		paths = append(paths, pc)
	}
	sort.Sort(paths)
	return paths, nil
}

func newHandler(config []byte) (*handler, error) {
	var parsed struct {
		Host      string          `yaml:"host,omitempty"`
		Paths     parsedPaths     `yaml:"paths,omitempty"`
		PathRules parsedPathRules `yaml:"pathrules,omitempty"`
	}
	if err := yaml.Unmarshal(config, &parsed); err != nil {
		return nil, err
	}
	h := &handler{host: parsed.Host}
	paths, err := parsePaths(parsed.Paths)
	if err != nil {
		return nil, err
	}
	h.paths = paths
	rules, err := parsePathRules(parsed.PathRules)
	if err != nil {
		return nil, err
	}
	h.pathRules = rules
	return h, nil
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	current := r.URL.Path
	pc, subpath := h.paths.find(current)
	if pc == nil {
		pc, subpath = h.pathRules.find(current)
	}
	if pc == nil && current == "/" {
		h.serveIndex(w, r)
		return
	}
	if pc == nil {
		http.NotFound(w, r)
		return
	}

	if err := vanityTmpl.Execute(w, struct {
		Import  string
		Subpath string
		Repo    string
		Display string
		VCS     string
	}{
		Import:  h.Host(r) + pc.path,
		Subpath: subpath,
		Repo:    pc.repo,
		Display: pc.display,
		VCS:     pc.vcs,
	}); err != nil {
		http.Error(w, "cannot render the page", http.StatusInternalServerError)
	}
}

func (h *handler) serveIndex(w http.ResponseWriter, r *http.Request) {
	host := h.Host(r)
	handlers := make([]string, len(h.paths))
	for i, h := range h.paths {
		handlers[i] = host + h.path
	}
	type GenericRule struct {
		Match string
		Subst string
	}
	if err := indexTmpl.Execute(w, struct {
		Host         string
		Handlers     []string
		GenericRules []GenericRule
	}{
		Host:     host,
		Handlers: handlers,
	}); err != nil {
		http.Error(w, "cannot render the page", http.StatusInternalServerError)
	}
}

func (h *handler) Host(r *http.Request) string {
	host := h.host
	if host == "" {
		host = defaultHost(r)
	}
	return host
}

var indexTmpl = template.Must(template.New("index").Parse(`<!DOCTYPE html>
<html>
<h1>{{.Host}}</h1>

<ul>
{{range .Handlers}}<li><a href="https://godoc.org/{{.}}">{{.}}</a></li>{{end}}
</ul>
<ul>
{{range .GenericRules}}<li>{{.Match}} will be clone repository {{.Subst}}></li>{{end}}
</ul>
</html>
`))

var vanityTmpl = template.Must(template.New("vanity").Parse(`<!DOCTYPE html>
<html>
<head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>
<meta name="go-import" content="{{.Import}} {{.VCS}} {{.Repo}}">
<meta name="go-source" content="{{.Import}} {{.Display}}">
<meta http-equiv="refresh" content="0; url=https://godoc.org/{{.Import}}/{{.Subpath}}">
</head>
<body>
Nothing to see here; <a href="https://godoc.org/{{.Import}}/{{.Subpath}}">see the package on godoc</a>.
</body>
</html>`))

type pathConfigRuleSet map[string]*pathConfigRule

func (prset pathConfigRuleSet) find(path string) (*pathConfig, string) {
	for prefix, rule := range prset {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		name := strings.TrimPrefix(path, prefix)
		name, subPath := splitSubpath(name)
		path = strings.TrimSuffix(path, subPath)
		repo := strings.Replace(rule.repoSubst, rule.placeholder, name, -1)
		display := strings.Replace(rule.display, rule.placeholder, name, -1)
		return &pathConfig{
			path:    path,
			repo:    repo,
			display: display,
			vcs:     rule.vcs,
		}, subPath
	}
	return nil, ""
}

// splitSubpath turn "foo/bar/baz" into ("foo", "/bar/baz")
func splitSubpath(name string) (string, string) {
	i := strings.Index(name, "/")
	if i < 0 {
		return name, ""
	}
	return name[:i], name[i:]
}

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
	i := sort.Search(len(pset), func(i int) bool {
		return pset[i].path >= path
	})
	if i < len(pset) && pset[i].path == path {
		return &pset[i], ""
	}
	if i > 0 && strings.HasPrefix(path, pset[i-1].path+"/") {
		return &pset[i-1], path[len(pset[i-1].path)+1:]
	}
	return nil, ""
}

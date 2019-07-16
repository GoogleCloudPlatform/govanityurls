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
	"net/http"
	"sort"
	"strings"

	yaml "gopkg.in/yaml.v2"
)

// Handler contains all the running data for our web server.
type Handler struct {
	*Config
	PathConfigs
}

// Config contains the config file data.
type Config struct {
	Host       string                 `yaml:"host,omitempty"`
	CacheAge   *uint64                `yaml:"cache_max_age,omitempty"`
	Paths      map[string]*PathConfig `yaml:"paths,omitempty"`
	RedirPaths []string               `yaml:"redir_paths,omitempty"`
}

// PathConfigs contains our list of configured routing-paths.
type PathConfigs []*PathConfig

// PathConfig is the configuration for a single routing path.
type PathConfig struct {
	Path         string   `yaml:"-"`
	CacheAge     *uint64  `yaml:"cache_max_age,omitempty"`
	RedirPaths   []string `yaml:"redir_paths,omitempty"`
	Repo         string   `yaml:"repo,omitempty"`
	Redir        string   `yaml:"redir,omitempty"`
	Display      string   `yaml:"display,omitempty"`
	VCS          string   `yaml:"vcs,omitempty"`
	cacheControl string
}

func newHandler(configData []byte) (*Handler, error) {
	h := &Handler{Config: &Config{Paths: make(map[string]*PathConfig)}}
	if err := yaml.Unmarshal(configData, h.Config); err != nil {
		return nil, err
	}
	cacheControl := fmt.Sprintf("public, max-age=86400") // 24 hours (in seconds)
	if h.CacheAge != nil {
		cacheControl = fmt.Sprintf("public, max-age=%d", *h.CacheAge)
	}
	for path, e := range h.Config.Paths {
		h.Config.Paths[path].Path = strings.TrimSuffix(path, "/")
		if len(e.RedirPaths) < 1 {
			e.RedirPaths = h.RedirPaths
		}
		h.Config.Paths[path].cacheControl = cacheControl
		if e.CacheAge != nil {
			h.Config.Paths[path].cacheControl = fmt.Sprintf("public, max-age=%d", *e.CacheAge)
		}

		switch {
		case e.Display != "":
			// Already filled in.
		case strings.HasPrefix(e.Repo, "https://github.com/"):
			h.Config.Paths[path].Display = fmt.Sprintf("%v %v/tree/master{/dir} %v/blob/master{/dir}/{file}#L{line}", e.Repo, e.Repo, e.Repo)
		case strings.HasPrefix(e.Repo, "https://bitbucket.org"):
			h.Config.Paths[path].Display = fmt.Sprintf("%v %v/src/default{/dir} %v/src/default{/dir}/{file}#{file}-{line}", e.Repo, e.Repo, e.Repo)
		}

		switch {
		case e.VCS != "":
			// Already filled in.
			if e.VCS != "bzr" && e.VCS != "git" && e.VCS != "hg" && e.VCS != "svn" {
				return nil, fmt.Errorf("configuration for %v: unknown VCS %s", path, e.VCS)
			}
		case strings.HasPrefix(e.Repo, "https://github.com/"):
			h.Config.Paths[path].VCS = "git"
		case e.Repo == "" && e.Redir != "":
			// Redirect-only can go anywhere.
		default:
			return nil, fmt.Errorf("configuration for %v: cannot infer VCS from %s", path, e.Repo)
		}

		h.PathConfigs = append(h.PathConfigs, e)
	}
	sort.Sort(h.PathConfigs)
	return h, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	current := r.URL.Path
	pc, subpath := h.PathConfigs.find(current)
	if pc == nil && current == "/" {
		if err := indexTmpl.Execute(w, &h.Config); err != nil {
			http.Error(w, "cannot render the page", http.StatusInternalServerError)
		}
		return
	}
	if pc == nil {
		http.NotFound(w, r)
		return
	}
	// Redirect for file downloads.
	if pc.Redir != "" && StringInSlices(subpath, pc.RedirPaths) {
		redirTo := pc.Redir + strings.TrimPrefix(current, pc.Path)
		http.Redirect(w, r, redirTo, 302)
		return
	}
	if pc.Repo == "" {
		// Repo is not set and no paths to redirect, so we're done.
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", pc.cacheControl)
	if err := vanityTmpl.Execute(w, struct {
		Host    string
		Subpath string
		*PathConfig
	}{
		Host:       h.Hostname(r),
		Subpath:    subpath,
		PathConfig: pc,
	}); err != nil {
		http.Error(w, "cannot render the page", http.StatusInternalServerError)
	}
}

// Hostname returns the appropriate Host header for this request.
func (h *Handler) Hostname(r *http.Request) string {
	if h.Host == "" {
		return defaultHost(r)
	}
	return h.Host
}

// StringInSlices checks if a string exists in a list of strings.
// Used to determine if a sub path shouuld be redirected or not.
func StringInSlices(str string, slice []string) bool {
	for _, s := range slice {
		if strings.Contains(str, s) {
			return true
		}
	}
	return false
}

// Len is a sort.Search interface method.
func (pset PathConfigs) Len() int {
	return len(pset)
}

// Less is a sort.Search interface method.
func (pset PathConfigs) Less(i, j int) bool {
	return pset[i].Path < pset[j].Path
}

// Swap is a sort.Search interface method.
func (pset PathConfigs) Swap(i, j int) {
	pset[i], pset[j] = pset[j], pset[i]
}

func (pset PathConfigs) find(path string) (pc *PathConfig, subpath string) {
	// Fast path with binary search to retrieve exact matches
	// e.g. given pset ["/", "/abc", "/xyz"], path "/def" won't match.
	i := sort.Search(len(pset), func(i int) bool {
		return pset[i].Path >= path
	})
	if i < len(pset) && pset[i].Path == path {
		return pset[i], ""
	}
	if i > 0 && strings.HasPrefix(path, pset[i-1].Path+"/") {
		return pset[i-1], path[len(pset[i-1].Path)+1:]
	}

	// Slow path, now looking for the longest prefix/shortest subpath i.e.
	// e.g. given pset ["/", "/abc/", "/abc/def/", "/xyz"/]
	//  * query "/abc/foo" returns "/abc/" with a subpath of "foo"
	//  * query "/x" returns "/" with a subpath of "x"
	lenShortestSubpath := len(path)
	var bestMatchConfig *PathConfig

	// After binary search with the >= lexicographic comparison,
	// nothing greater than i will be a prefix of path.
	max := i
	for i := 0; i < max; i++ {
		ps := pset[i]
		if len(ps.Path) >= len(path) {
			// We previously didn't find the path by search, so any
			// route with equal or greater length is NOT a match.
			continue
		}
		sSubpath := strings.TrimPrefix(path, ps.Path)
		if len(sSubpath) < lenShortestSubpath {
			subpath = sSubpath
			lenShortestSubpath = len(sSubpath)
			bestMatchConfig = pset[i]
		}
	}
	return bestMatchConfig, subpath
}

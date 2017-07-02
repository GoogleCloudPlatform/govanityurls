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
	"strings"

	"gopkg.in/yaml.v2"
)

type handler struct {
	m map[string]struct {
		Repo    string `yaml:"repo,omitempty"`
		Display string `yaml:"display,omitempty"`
	}
}

func newHandler(config []byte) (*handler, error) {
	h := new(handler)
	if err := yaml.Unmarshal(config, &h.m); err != nil {
		return nil, err
	}
	for _, e := range h.m {
		if e.Display != "" {
			continue
		}
		if strings.Contains(e.Repo, "github.com") {
			e.Display = fmt.Sprintf("%v %v/tree/master{/dir} %v/blob/master{/dir}/{file}#L{line}", e.Repo, e.Repo, e.Repo)
		}
	}
	return h, nil
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	current := r.URL.Path
	p, ok := h.m[current]
	if !ok {
		http.NotFound(w, r)
		return
	}

	host := requestHost(r)
	if err := vanityTmpl.Execute(w, struct {
		Import  string
		Repo    string
		Display string
	}{
		Import:  host + current,
		Repo:    p.Repo,
		Display: p.Display,
	}); err != nil {
		http.Error(w, "cannot render the page", http.StatusInternalServerError)
	}
}

var vanityTmpl = template.Must(template.New("vanity").Parse(`<!DOCTYPE html>
<html>
<head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8"/>
<meta name="go-import" content="{{.Import}} git {{.Repo}}">
<meta name="go-source" content="{{.Import}} {{.Display}}">
<meta http-equiv="refresh" content="0; url=https://godoc.org/{{.Import}}">
</head>
<body>
Nothing to see here; <a href="https://godoc.org/{{.Import}}">see the package on godoc</a>.
</body>
</html>`))

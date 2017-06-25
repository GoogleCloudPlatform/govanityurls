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

// Package main contains an App Engine that serves vanity URLs for a git repo.
package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"

	"google.golang.org/appengine"

	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type ypkg struct {
	Repo    string `yaml:"repo,omitempty"`
	Display string `yaml:"display,omitempty"`
}

var m map[string]ypkg

func init() {
	vanity, err := ioutil.ReadFile("./vanity.yaml")
	if err != nil {
		log.Fatal(err)
	}
	if err := yaml.Unmarshal(vanity, &m); err != nil {
		log.Fatal(err)
	}
	for _, e := range m {
		if e.Display != "" {
			continue
		}
		if strings.Contains(e.Repo, "github.com") {
			e.Display = fmt.Sprintf("%v %v/tree/master{/dir} %v/blob/master{/dir}/{file}#L{line}", e.Repo, e.Repo, e.Repo)
		}
	}
	http.HandleFunc("/", handle)
}

func handle(w http.ResponseWriter, r *http.Request) {
	current := r.URL.Path
	p, ok := m[current]
	if !ok {
		http.NotFound(w, r)
		return
	}

	host := appengine.DefaultVersionHostname(appengine.NewContext(r))
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

var vanityTmpl, _ = template.New("vanity").Parse(`<!DOCTYPE html>
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
</html>`)

func mustLoad(key string, value *string) {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing %v env variable", key)
	}
	*value = v
}

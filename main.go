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
)

type pkg struct {
	Import     string
	Repo       string
	Display    string
	CurrentPkg string
}

var (
	domain  string
	repo    string
	display string
)

func init() {
	mustLoad("DOMAIN", &domain)
	mustLoad("REPO", &repo)
	if strings.Contains(repo, "github.com") {
		display = fmt.Sprintf("%v %v/tree/master{/dir} %v/blob/master{/dir}/{file}#L{line}", repo, repo, repo)
	}
	if display == "" {
		mustLoad("DISPLAY", &display)
	}
	http.HandleFunc("/", handle)
}

func handle(w http.ResponseWriter, r *http.Request) {
	current := r.URL.Path
	if err := vanityTmpl.Execute(w, &pkg{
		Import:     domain,
		Repo:       repo,
		Display:    display,
		CurrentPkg: current,
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
<meta http-equiv="refresh" content="0; url=https://godoc.org/{{.Import}}{{.CurrentPkg}}">
</head>
<body>
Nothing to see here; <a href="https://godoc.org/{{.Import}}{{.CurrentPkg}}">move along</a>.
</body>
</html>`)

func mustLoad(key string, value *string) {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing %v env variable", key)
	}
	*value = v
}

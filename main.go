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

//+build !appengine

package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

func main() {
	var configPath string
	switch len(os.Args) {
	case 1:
		configPath = "vanity.yaml"
	case 2:
		configPath = os.Args[1]
	default:
		log.Fatal("usage: govanityurls [CONFIG]")
	}
	vanity, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatal(err)
	}
	h, err := newHandler(vanity)
	if err != nil {
		log.Fatal(err)
	}
	http.Handle("/", h)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func defaultHost(r *http.Request) string {
	return r.Host
}

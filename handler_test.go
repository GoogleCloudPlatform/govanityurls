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

package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler(t *testing.T) {
	tests := []struct {
		name   string
		config string
		path   string

		goImport string
		goSource string
	}{
		{
			name: "explicit display",
			config: "/portmidi:\n" +
				"  repo: https://github.com/rakyll/portmidi\n" +
				"  display: https://github.com/rakyll/portmidi _ _\n",
			path:     "/portmidi",
			goImport: "example.com/portmidi git https://github.com/rakyll/portmidi",
			goSource: "example.com/portmidi https://github.com/rakyll/portmidi _ _",
		},
		{
			name: "display GitHub inference",
			config: "/portmidi:\n" +
				"  repo: https://github.com/rakyll/portmidi\n",
			path:     "/portmidi",
			goImport: "example.com/portmidi git https://github.com/rakyll/portmidi",
			goSource: "example.com/portmidi https://github.com/rakyll/portmidi https://github.com/rakyll/portmidi/tree/master{/dir} https://github.com/rakyll/portmidi/blob/master{/dir}/{file}#L{line}",
		},
		{
			name: "Bitbucket",
			config: "/gopdf:\n" +
				"  repo: https://bitbucket.org/zombiezen/gopdf\n",
			path:     "/gopdf",
			goImport: "example.com/gopdf hg https://bitbucket.org/zombiezen/gopdf",
			goSource: "example.com/gopdf https://bitbucket.org/zombiezen/gopdf https://bitbucket.org/zombiezen/gopdf/src/default{/dir} https://bitbucket.org/zombiezen/gopdf/src/default{/dir}/{file}#{file}-{line}",
		},
	}
	for _, test := range tests {
		h, err := newHandler([]byte(test.config))
		if err != nil {
			t.Errorf("%s: newHandler: %v", test.name, err)
			continue
		}
		h.host = "example.com"
		s := httptest.NewServer(h)
		resp, err := http.Get(s.URL + test.path)
		if err != nil {
			s.Close()
			t.Errorf("%s: http.Get: %v", test.name, err)
			continue
		}
		data, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		s.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: status code = %s; want 200 OK", test.name, resp.Status)
		}
		if err != nil {
			t.Errorf("%s: ioutil.ReadAll: %v", test.name, err)
			continue
		}
		if got := findMeta(data, "go-import"); got != test.goImport {
			t.Errorf("%s: meta go-import = %q; want %q", test.name, got, test.goImport)
		}
		if got := findMeta(data, "go-source"); got != test.goSource {
			t.Errorf("%s: meta go-source = %q; want %q", test.name, got, test.goSource)
		}
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

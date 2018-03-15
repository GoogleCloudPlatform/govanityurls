// Copyright 2018 Google Inc. All Rights Reserved.
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
	"errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping due to -test.short")
	}
	goExe, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("Could not find go tool: %v", err)
	}

	var gitURLArgv []string
	var hgURLArgv []string
	if gitExe, err := exec.LookPath("git"); err != nil {
		t.Logf("Could not find git: %v", err)
	} else {
		gitURLArgv = []string{gitExe, "remote", "get-url", "origin"}
	}
	if hgExe, err := exec.LookPath("hg"); err != nil {
		t.Logf("Could not find hg: %v", err)
	} else {
		hgURLArgv = []string{hgExe, "paths", "default"}
	}
	if gitURLArgv == nil && hgURLArgv == nil {
		t.Skip("Could not find any VCS; skipping")
	}
	tests := []struct {
		name       string
		config     string
		importPath string
		getURLArgv []string
		wantURL    string
	}{
		{
			name: "GitHub",
			config: "host: example.com\n" +
				"paths:\n" +
				"  /portmidi:\n" +
				"    repo: https://github.com/rakyll/portmidi\n",
			importPath: "example.com/portmidi",
			getURLArgv: gitURLArgv,
			wantURL:    "https://github.com/rakyll/portmidi",
		},
		{
			name: "Bitbucket Mercurial",
			config: "host: example.com\n" +
				"paths:\n" +
				"  /gopdf:\n" +
				"    repo: https://bitbucket.org/zombiezen/gopdf\n" +
				"    vcs: hg\n",
			importPath: "example.com/gopdf/pdf",
			getURLArgv: hgURLArgv,
			wantURL:    "https://bitbucket.org/zombiezen/gopdf",
		},
		{
			name: "Bitbucket Git",
			config: "host: example.com\n" +
				"paths:\n" +
				"  /cardcpx:\n" +
				"    repo: https://bitbucket.org/zombiezen/cardcpx.git\n" +
				"    vcs: git\n",
			importPath: "example.com/cardcpx/natsort",
			getURLArgv: gitURLArgv,
			wantURL:    "https://bitbucket.org/zombiezen/cardcpx.git",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.getURLArgv == nil {
				t.Skip("VCS tool not installed; skipping")
			}
			h, err := newHandler([]byte(test.config))
			if err != nil {
				t.Fatal(err)
			}
			tempRoot, err := ioutil.TempDir("", "govanityurls_integration")
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.RemoveAll(tempRoot); err != nil {
					t.Error(err)
				}
			}()
			tempDir := filepath.Join(tempRoot, "tmp")
			if err := os.Mkdir(tempDir, 0777); err != nil {
				t.Fatal(err)
			}
			cacheDir := filepath.Join(tempRoot, "cache")
			if err := os.Mkdir(cacheDir, 0777); err != nil {
				t.Fatal(err)
			}
			gopathDir := filepath.Join(tempRoot, "gopath")
			if err := os.Mkdir(gopathDir, 0777); err != nil {
				t.Fatal(err)
			}
			srv := httptest.NewServer(&proxy{
				rt:      http.DefaultTransport,
				host:    "example.com",
				handler: h,
			})
			defer srv.Close()
			goCmd := exec.Command(goExe, "get", "-insecure", "-d", test.importPath)
			goCmd.Env = appendEnv(os.Environ(),
				"GOPATH="+gopathDir,
				"HTTP_PROXY="+srv.URL,
				"TMPDIR="+tempDir,
				// Go 1.10+ environment variables:
				"GOCACHE="+cacheDir,
				"GOTMPDIR="+tempDir)
			getOutput, err := goCmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go get failed. log:\n%s", getOutput)
			}

			vcsStderr := new(bytes.Buffer)
			vcsStderrLimiter := &limitedWriter{w: vcsStderr, n: 2048}
			vcsCmd := exec.Cmd{
				Path: test.getURLArgv[0],
				Args: test.getURLArgv,
				Env: appendEnv(os.Environ(),
					"HOME="+tempRoot,
					"TMPDIR="+tempDir,
					"XDG_CONFIG_HOME="+filepath.Join(tempRoot, ".config")), // intentionally does not exist
				Dir:    filepath.Join(gopathDir, "src", filepath.FromSlash(test.importPath)),
				Stderr: vcsStderrLimiter,
			}
			got, err := vcsCmd.Output()
			if err != nil {
				format := "%s failed. log:\n%s"
				if vcsStderrLimiter.truncated {
					format += "<truncated>"
				}
				t.Fatalf(format, strings.Join(test.getURLArgv, " "), vcsStderr.Bytes())
			}
			if want := []byte(test.wantURL + "\n"); !bytes.Equal(got, want) {
				t.Errorf("%s = %q; want %q", strings.Join(test.getURLArgv, " "), got, want)
			}
		})
	}
}

// proxy is an HTTP proxy that forwards requests to a certain host to
// another handler.
type proxy struct {
	rt http.RoundTripper

	host    string
	handler http.Handler
}

func (p *proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		// Just "host", without port.
		host = r.URL.Host
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead:
		if host == p.host {
			p.handler.ServeHTTP(w, r)
			return
		}
		res, err := p.rt.RoundTrip(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer res.Body.Close()
		for k, v := range res.Header {
			w.Header()[k] = append([]string(nil), v...)
		}
		w.WriteHeader(res.StatusCode)
		io.Copy(w, res.Body)
	case http.MethodConnect:
		if host == p.host {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "test proxy only allows GET and HEAD to "+r.Host, http.StatusMethodNotAllowed)
			return
		}
		p.connect(w, r)
	default:
		w.Header().Set("Allow", "GET, HEAD, CONNECT")
		http.Error(w, "test proxy only allows GET, HEAD, and CONNECT", http.StatusMethodNotAllowed)
	}
}

func (p *proxy) connect(w http.ResponseWriter, r *http.Request) {
	c1, err := new(net.Dialer).DialContext(r.Context(), "tcp", r.Host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	hi, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "could convert HTTP connection to TCP", http.StatusInternalServerError)
		return
	}
	c2, brw, err := hi.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c2.SetDeadline(time.Time{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		buf, _ := brw.Reader.Peek(brw.Reader.Buffered())
		c1.Write(buf)
		io.Copy(c1, c2)
	}()
	go func() {
		defer wg.Done()
		io.Copy(c2, c1)
	}()
	wg.Wait()
	c1.Close()
	c2.Close()
}

// appendEnv returns a new environment variable list that overrides
// environment variables from base.
//
// Before Go 1.9, os/exec.Cmd.Env would not be deduplicated.
func appendEnv(base []string, add ...string) []string {
	out := make([]string, 0, len(base)+len(add))
	saw := make(map[string]int)
	insert := func(kv string) {
		eq := strings.IndexByte(kv, '=')
		if eq < 0 {
			out = append(out, kv)
			return
		}
		k := kv[:eq]
		if dupIdx, isDup := saw[k]; isDup {
			out[dupIdx] = kv
			return
		}
		saw[k] = len(out)
		out = append(out, kv)
	}
	for _, kv := range base {
		insert(kv)
	}
	for _, kv := range add {
		insert(kv)
	}
	return out
}

type limitedWriter struct {
	w         io.Writer
	n         int64
	truncated bool
}

func (l *limitedWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	if l.n <= 0 {
		l.truncated = true
		return 0, errors.New("reached write limit")
	}
	if int64(len(p)) > l.n {
		l.truncated = true
		n, err = l.w.Write(p[:l.n])
		l.n -= int64(n)
		if err != nil {
			return n, err
		}
		return n, errors.New("reached write limit")
	}
	n, err = l.w.Write(p)
	l.n -= int64(n)
	return n, err
}

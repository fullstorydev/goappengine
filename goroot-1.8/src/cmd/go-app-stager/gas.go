// Copyright 2016 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command appengine-go-staging stages an App Engine Standard Go app,
// according to the staging protocol specified in the Google Cloud SDK, under
// `command_lib/app/staging.py`.
//
// Usage: go-app-stager SERVICE_YAML STAGED_DIR
// Stdout: Path to staged SERVICE_YAML
// Stderr: All debug and errors
//
// SERVICE_YAML is the path to the original `<service>.yaml` (commonly
// `app.yaml`) from the unstaged app directory (and is left untouched).
//
// STAGED_DIR should be an empty directory, and is populated by this command.
package main

import (
	"flag"
	"fmt"
	"go/build"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"appengine_internal/gopkg.in/yaml.v2"
)

func usage() {
	fmt.Fprint(os.Stderr, `Usage of go-app-stager:
	go-app-stager SERVICE_YAML APP_DIR STAGED_DIR

	Stage App Engine app in an empty directory.
	SERVICE_YAML:	Path to original '<service>.yaml' file, (app.yaml)
	APP_DIR:	Path to original app directory (usually contains SERVICE_YAML, but not a requirement)
	STAGED_DIR:	Path to an empty directory where the app should be staged
`)
}

// Top-level standard library packages, used instead of depending on a Goroot.
var skippedPackages = map[string]bool{
	"appengine":          true,
	"appengine_internal": true,
	"C":                  true,
	"unsafe":             true,

	"archive":   true,
	"bufio":     true,
	"builtin":   true,
	"bytes":     true,
	"compress":  true,
	"container": true,
	"context":   true,
	"crypto":    true,
	"database":  true,
	"debug":     true,
	"encoding":  true,
	"errors":    true,
	"expvar":    true,
	"flag":      true,
	"fmt":       true,
	"go":        true,
	"hash":      true,
	"html":      true,
	"image":     true,
	"index":     true,
	"io":        true,
	"log":       true,
	"math":      true,
	"mime":      true,
	"net":       true,
	"os":        true,
	"path":      true,
	"plugin":    true,
	"reflect":   true,
	"regexp":    true,
	"runtime":   true,
	"sort":      true,
	"strconv":   true,
	"strings":   true,
	"sync":      true,
	"syscall":   true,
	"testing":   true,
	"text":      true,
	"time":      true,
	"unicode":   true,
}

// Subset of <service>.yaml (commonly app.yaml)
type config struct {
	VM  bool   `yaml:"vm"`
	Env string `yaml:"env"`
}

func (conf *config) isFlex() bool {
	return conf.VM || conf.Env == "flex" || conf.Env == "flexible" || conf.Env == "2"
}

type importFrom struct {
	path    string
	fromDir string
}

var (
	skipFiles = map[string]bool{
		".git":        true,
		".gitconfig":  true,
		".hg":         true,
		".travis.yml": true,
	}
	minorVersions = []int{6, 7, 8, 9} // go1.n, both flex and standard
)

func main() {
	flag.Parse()
	if narg := flag.NArg(); narg != 3 {
		usage()
		flag.PrintDefaults()
		os.Exit(1)
	}
	// Path to the <service>.yaml file
	configPath := flag.Arg(0)
	src := flag.Arg(1)
	dst := flag.Arg(2)

	// Read and parse app.yaml file
	var c config
	contents, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Printf("failed to read %s: %v", configPath, err)
		os.Exit(1)
	}
	if err = yaml.Unmarshal(contents, &c); err != nil {
		log.Printf("failed to unmarshal YAML config: %v", err)
		os.Exit(1)
	}

	// Get deps for dir []string
	tags := []string{"appengine"}
	enforceMain := false
	dstDepsDir := ""
	if c.isFlex() {
		tags = []string{"appenginevm"}
		enforceMain = true
		dstDepsDir = filepath.Join("_gopath", "src")
		skippedPackages["appengine"] = false // Doesn't exist for flex
		mainPathFile := filepath.Join(dst, "_gopath", "main-package-path")
		if err := writeMainPkgFile(mainPathFile, src); err != nil {
			log.Printf("failed to write _gopath/main-package-path: %v", err)
			os.Exit(1)
		}
	}
	// Multipass analyzing in order to respect release tags,
	for _, minorVersion := range minorVersions {
		buildCtx := buildContext(tags, minorVersion)
		deps, err := analyze(src, buildCtx, enforceMain)
		if err != nil {
			log.Printf("failed analyzing %s: %v\nGOPATH: %s\n", src, err, buildCtx.GOPATH)
			os.Exit(1)
		}
		if err = bundle(dst, dstDepsDir, deps); err != nil {
			log.Printf("failed to bundle to %s: %v", dst, err)
			os.Exit(1)
		}
	}
	if err = copyTree(dst, ".", src, true); err != nil {
		log.Printf("unable to copy root directory to /app: %v", err)
		os.Exit(1)
	}
}

// writeMainPkgFile writes out the main package path relative to GOPATH into given file.
// It determines the path based on given appDir, which is the original app directory. It will find
// the GOPATH entry where appDir is a subdirectory.  If no GOPATH entry is found, it means that the
// main package path is outside of GOPATH and will simply return nil without writing to the given
// file.
//
// The main package path will be used by the builder to recreate the relative path for building the
// app.  This ensures that vendor and internal directories are properly accounted for.  Without a
// main package path, the builder does a build of the app outside of GOPATH.
func writeMainPkgFile(file string, appDir string) error {
	mainDir, err := filepath.Abs(appDir)
	if err != nil {
		return fmt.Errorf("could not get absolute path for dir %q: %v", appDir, err)
	}

	// Find the GOPATH entry that contains the main package path and set mainPath. Use
	// build.Default.GOPATH which uses either the GOPATH env or the OS-specific default.
	// TODO: Pass in GOPATH value based on the build.Context used in analyze.
	mainPath := ""
	for _, p := range filepath.SplitList(build.Default.GOPATH) {
		gop, err := filepath.Abs(p)
		if err != nil {
			return fmt.Errorf("could not get absolute path for GOPATH entry %q: %v", p, err)
		}
		srcDir := filepath.Join(gop, "src") + string(filepath.Separator)
		if strings.HasPrefix(mainDir, srcDir) {
			mainPath = strings.TrimPrefix(mainDir, srcDir)
			break
		}
	}

	// No GOPATH entry contains the main package path.
	if mainPath == "" {
		return nil
	}

	dstDir := filepath.Dir(file)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("unable to create directory %q: %v", dstDir, err)
	}

	// Write out mainPath to file.
	f, err := os.Create(file)
	if err != nil {
		return fmt.Errorf("unable to create %q: %v", file, err)
	}
	if _, err := f.WriteString(mainPath); err != nil {
		return fmt.Errorf("unable to write %q: %v", file, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("unable to close %q: %v", file, err)
	}
	fmt.Fprintf(os.Stderr, "main-package: %s\n", mainPath)
	return nil
}

// buildContext returns the context for building the source.
func buildContext(tags []string, minorVersion int) *build.Context {
	ctx := &build.Context{
		GOARCH:      "amd64",
		GOOS:        "linux",
		GOROOT:      "",
		GOPATH:      build.Default.GOPATH,
		Compiler:    build.Default.Compiler,
		BuildTags:   tags,
		ReleaseTags: releaseTags(minorVersion),
	}
	return ctx
}

func releaseTags(minorVersion int) []string {
	var tags []string
	for i := 1; i <= minorVersion; i++ {
		tags = append(tags, fmt.Sprintf("go1.%d", i))
	}
	return tags
}

// enforceMain, if not main will return an error.
func analyze(dir string, ctx *build.Context, enforceMain bool) ([]*build.Package, error) {
	visited := make(map[importFrom]bool)
	var imports []importFrom
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("could not get absolute path for dir %q: %v", dir, err)
	}
	pkg, err := ctx.ImportDir(abs, 0)
	if err != nil {
		return nil, fmt.Errorf("could not get package for dir %q: %v", dir, err)
	}
	if enforceMain && !pkg.IsCommand() {
		return nil, fmt.Errorf(`the root of your app needs to be package "main" (currently %q)`, pkg.Name)
	}
	for _, importPath := range pkg.Imports {
		imports = append(imports, importFrom{
			path:    importPath,
			fromDir: abs,
		})
	}
	packages := make([]*build.Package, 0)
	visitedPackages := make(map[string]bool)
	for len(imports) != 0 {
		i := imports[0]
		imports = imports[1:] // shift

		if _, ok := visited[i]; ok {
			continue
		}
		// Handle skipped packages
		firstPart := strings.SplitN(i.path, "/", 2)[0]
		if ok, _ := skippedPackages[firstPart]; ok { // Part of stdlib
			continue
		}
		visited[i] = true
		pkg, err := ctx.Import(i.path, i.fromDir, 0)
		if err != nil {
			return nil, err
		}
		name := filepath.Join(pkg.SrcRoot, pkg.ImportPath)
		if _, ok := visitedPackages[name]; !ok {
			visitedPackages[name] = true
			packages = append(packages, pkg)
		}
		// Recursively add new imports
		for _, importPath := range pkg.Imports {
			imports = append(imports, importFrom{
				path:    importPath,
				fromDir: pkg.Dir,
			})
		}
	}
	return packages, nil
}

// bundle copies package dependencies to staged _gopath/src/.
func bundle(dst, dstDepsDir string, deps []*build.Package) error {
	for _, pkg := range deps {
		dstDir := filepath.Join(dstDepsDir, pkg.ImportPath)
		srcDir := filepath.Join(pkg.SrcRoot, pkg.ImportPath)
		if err := copyTree(dst, dstDir, srcDir, false); err != nil {
			return fmt.Errorf("unable to copy directory %v to %v: %v", srcDir, dstDir, err)
		}
	}
	return nil
}

// copyTree copies srcDir to dstDir relative to dstRoot, ignoring skipFiles.
func copyTree(dstRoot, dstDir, srcDir string, recursive bool) error {
	d := filepath.Join(dstRoot, dstDir)
	if err := os.MkdirAll(d, 0755); err != nil {
		return fmt.Errorf("unable to create directory %q: %v", d, err)
	}

	entries, err := ioutil.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("unable to read dir %q: %v", srcDir, err)
	}
	for _, entry := range entries {
		n := entry.Name()
		s := filepath.Join(srcDir, n)
		if skipFiles[n] {
			fmt.Fprintf(os.Stderr, "skipping %s\n", s)
			continue
		}
		if entry.Mode()&os.ModeSymlink == os.ModeSymlink {
			if entry, err = os.Stat(s); err != nil {
				return fmt.Errorf("unable to stat %v: %v", s, err)
			}
		}
		d := filepath.Join(dstDir, n)
		if entry.IsDir() {
			if !recursive {
				continue
			}
			if err := copyTree(dstRoot, d, s, recursive); err != nil {
				return fmt.Errorf("unable to copy dir %q to %q: %v", s, d, err)
			}
			continue
		}
		if err := copyFile(dstRoot, d, s); err != nil {
			return fmt.Errorf("unable to copy dir %q to %q: %v", s, d, err)
		}
		fmt.Fprintf(os.Stderr, "copied %s to %s\n", s, filepath.Join(dstRoot, d))
	}
	return nil
}

// copyFile copies src to dst relative to dstRoot.
func copyFile(dstRoot, dst, src string) error {
	s, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("unable to open %q: %v", src, err)
	}
	defer s.Close()

	dst = filepath.Join(dstRoot, dst)
	d, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("unable to create %q: %v", dst, err)
	}
	_, err = io.Copy(d, s)
	if err != nil {
		d.Close() // ignore error, copy already failed.
		return fmt.Errorf("unable to copy %q to %q: %v", src, dst, err)
	}
	if err := d.Close(); err != nil {
		return fmt.Errorf("unable to close %q: %v", dst, err)
	}
	return nil
}

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

// Command go-app-stager stages an App Engine Standard/Flexible Go app,
// according to the staging protocol specified in the Google Cloud SDK, under
// `command_lib/app/staging.py`.  It will stage the app for a given Go version.
//
// For GAE Standard, the Go version can be specified in the app.yaml file's
// api_version field with value in the form of `go1.x[RC]`.  If api_version
// field has unpinned version value of `go1`, use the constant
// stdDefaultMinorVersion defined below.  If api_version field is not set or not
// a valid value, go-app-stager will error out.
//
// For GAE Flex, gcloud will set GAE_RUNTIME_NAME environment variable to a
// resolved runtime value based on runtimes.yaml file, i.e. if value is `go`,
// gcloud should resolve it to a runtime value of `go1.x[RC]` format.  gcloud
// may provide other runtime values, e.g. `custom`, in which case user should
// use the flag -go-version to set the Go version to stage with, otherwise
// go-app-stager will error out.
//
// TODO: gcloud currently does not set GAE_RUNTIME_NAME yet, and hence
// go-app-stager falls back to parsing app.yaml for runtime value on its own
// for now and use the constant flexDefaultMinorVersion if value is `go`.  Once
// gcloud sets GAE_RUNTIME_NAME environment variable, this fallback logic can
// be removed.
//
// Current codebase assumes 1.x versions even though its interface allows for
// versions beyond Go1.
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
	"strconv"
	"strings"

	"appengine_internal/gopkg.in/yaml.v2"
)

const (
	stdDefaultMinorVersion  = 8
	flexDefaultMinorVersion = 10
)

func init() {
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage:
  go-app-stager [-go-version=x.y] SERVICE_YAML APP_DIR STAGED_DIR

  Stage App Engine app into STAGED_DIR.
  SERVICE_YAML: Path to original '<service>.yaml' file, (app.yaml)
  APP_DIR:      Path to original app directory (usually contains SERVICE_YAML, but not a requirement)
  STAGED_DIR:   Path to an empty directory where the app should be staged

`)
		flag.PrintDefaults()
	}
}

// Go version flag to use for finding dependencies. If set, it overrides all other options.
var goVersion = flag.String("go-version", "", "target Go release version, e.g. 1.8")

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
	Runtime    string `yaml:"runtime"`
	VM         bool   `yaml:"vm"`
	Env        string `yaml:"env"`
	APIVersion string `yaml:"api_version"`
}

func (conf *config) isStandard() bool {
	return conf.Runtime == "go" && conf.APIVersion != ""
}

func (conf *config) isFlex() bool {
	return !conf.isStandard()
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
)

func main() {
	flag.Parse()
	if narg := flag.NArg(); narg != 3 {
		flag.Usage()
		os.Exit(1)
	}
	// Path to the <service>.yaml file
	configPath := flag.Arg(0)
	src := flag.Arg(1)
	dst := flag.Arg(2)

	// Read and parse app.yaml file
	c, err := readConfig(configPath)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	// Determine Go minor version to use.
	minorVer, err := minorVersion(c, *goVersion)
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
	log.Printf("staging for go1.%d", minorVer)

	tags := []string{"appengine", "purego"}
	enforceMain := false
	dstDepsDir := ""
	if c.isFlex() {
		tags = []string{"appenginevm"}
		enforceMain = true
		dstDepsDir = filepath.Join("_gopath", "src")
		skippedPackages["appengine"] = false // Doesn't exist for flex

		// Write out _gopath/main-package-path if main package is under GOPATH.
		mainPathFile := filepath.Join(dst, "_gopath", "main-package-path")
		if err := writeMainPkgFile(mainPathFile, src); err != nil {
			log.Printf("failed to write _gopath/main-package-path: %v", err)
			os.Exit(1)
		}
	}

	// Find all dependencies for a build.Context for the release version and bundle their
	// directories into the staged directory.
	buildCtx := buildContext(tags, minorVer)
	deps, err := analyze(src, buildCtx, enforceMain)
	if err != nil {
		log.Printf("failed analyzing %s: %v\nGOPATH: %s\n", src, err, buildCtx.GOPATH)
		os.Exit(1)
	}
	if err = bundle(dst, dstDepsDir, deps); err != nil {
		log.Printf("failed to bundle to %s: %v", dst, err)
		os.Exit(1)
	}
	if err = copyTree(dst, ".", src, true); err != nil {
		log.Printf("unable to copy root directory to /app: %v", err)
		os.Exit(1)
	}
}

// readConfig parses given app.yaml file path.
func readConfig(path string) (*config, error) {
	c := &config{}
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %v", path, err)
	}
	if err = yaml.Unmarshal(contents, c); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML config: %v", err)
	}
	return c, nil
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

func minorVersion(cfg *config, fval string) (int, error) {
	// Use flag value first if set.
	if fval != "" {
		if mv, ok := parseMinorVersion(fval, "1."); ok {
			return mv, nil
		}
		return 0, fmt.Errorf("invalid -go-version flag value: %s", fval)
	}
	// Use either Flex or Standard specific logic at determining version.
	if cfg.isFlex() {
		return flexMinorVersion(cfg)
	}
	return stdMinorVersion(cfg)
}

// stdMinorVersion returns minor version for GAE Standard.
func stdMinorVersion(cfg *config) (int, error) {
	val := cfg.APIVersion
	if val == "go1" {
		return stdDefaultMinorVersion, nil
	}
	mv, ok := parseGo1MinorVersion(val)
	if !ok {
		// Invalid value.
		return -1, fmt.Errorf("invalid api_version value %s", val)
	}
	return mv, nil
}

// flexMinorVersion returns minor version for GAE Flex.
func flexMinorVersion(cfg *config) (int, error) {
	// Check environment variable.
	if ev := os.Getenv("GAE_RUNTIME_NAME"); ev != "" {
		if mv, ok := parseGo1MinorVersion(ev); ok {
			return mv, nil
		}
		return -1, fmt.Errorf("unable to stage for GAE_RUNTIME_NAME %q", ev)
	}
	// TODO: Remove logic below once gcloud sets GAE_RUNTIME_NAME.
	// Check app.yaml.
	val := cfg.Runtime
	if strings.HasSuffix(val, "go") {
		return flexDefaultMinorVersion, nil
	}
	mv, ok := parseGo1MinorVersion(val)
	if !ok {
		return -1, fmt.Errorf("unable to stage for runtime %q", cfg.Runtime)
	}
	return mv, nil
}

func parseGo1MinorVersion(val string) (int, bool) {
	// For version value of `go1.xRC`, treat as `go1.x`.
	val = strings.TrimSuffix(val, "RC")
	return parseMinorVersion(val, "go1.")
}

func parseMinorVersion(val string, prefix string) (int, bool) {
	if !strings.HasPrefix(val, prefix) {
		return 0, false
	}
	s := strings.TrimPrefix(val, prefix)
	mv, err := strconv.Atoi(s)
	return mv, err == nil
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

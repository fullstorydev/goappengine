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
// For GAE Standard second-gen, the Go version must be specified in app.yaml's
// runtime field with value in the form of `go1XX'. For example, `go111`. There
// is no default runtime version for GAE Standard second-gen. This runtime
// supports Go modules defined via a go.mod file.
//
// For GAE Standard, the Go version can be specified in the app.yaml file's
// api_version field with value in the form of `go1.x[RC]`.  If api_version
// field has unpinned version value of `go1`, use the constant
// stdDefaultMinorVersion defined below.  If api_version field is not set or not
// a valid value, go-app-stager will error out.
//
// For GAE Flex, the Go version can be specified in the app.yaml file's runtime
// field with value in the form of `go1.x[RC]`.  If runtime field has unpinned
// version value of `go`, it will determine the version to use from
// flexRuntimesConfigURL.
//
// go-app-stager can be invoked with a specific Go version via the -go-version
// flag to override the logic above.
//
// Current codebase assumes Go 1.x versions.
package main

import (
	"flag"
	"fmt"
	"go/build"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"appengine_internal/gopkg.in/yaml.v2"
)

const stdDefaultMinorVersion = 9

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

// Flag to override URL for Flex staging logic to fetch runtimes.yaml file.
var flexRuntimesConfigURL = flag.String("flex-runtimes-url",
	"http://storage.googleapis.com/runtime-builders/runtimes.yaml", "Flex runtimes.yaml URL")

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

func (conf *config) isFlex() bool {
	return conf.VM || conf.Env == "flex" || conf.Env == "flexible" || conf.Env == "2"
}

func (conf *config) isLegacyStandard() bool {
	return !conf.isFlex() && conf.Runtime == "go"
}

func (conf *config) isStandardSecondGen() bool {
	return !conf.isFlex() && strings.HasPrefix(conf.Runtime, "go1")
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

	tags := []string{}
	if c.isLegacyStandard() {
		tags = []string{"appengine", "purego"}
	} else if c.isFlex() {
		tags = []string{"appenginevm"}
	}
	buildCtx := buildContext(tags, minorVer)
	switch {
	case c.isLegacyStandard():
		if err := stageLegacyStandard(src, dst, buildCtx); err != nil {
			log.Fatalf("Staging Standard app: %s\n", err)
		}
	case c.isFlex():
		if err := stageFlex(src, dst, buildCtx); err != nil {
			log.Fatalf("Staging Flex app: %s\n", err)
		}
	case c.isStandardSecondGen():
		if err := stageStandardSecondGen(src, dst, buildCtx); err != nil {
			log.Fatalf("Staging second-gen Standard app: %s\n", err)
		}
	default:
		log.Fatalf("Unrecognized runtime: %s\n", c.Runtime)
	}
}

// stageLegacyStandard Stages a legacy GAE Standard app. Does not supporting vendoring or modules.
func stageLegacyStandard(src, dst string, buildCtx *build.Context) error {
	// Find all dependencies for a build.Context for the release version and bundle their
	// directories into the staged directory.
	deps, err := analyze(src, buildCtx, false /* enforceMain */)
	if err != nil {
		return fmt.Errorf("failed analyzing %s: %v\nGOPATH: %s", src, err, buildCtx.GOPATH)
	}
	if err = bundle(dst, "", deps); err != nil {
		return fmt.Errorf("failed to bundle to %s: %v", dst, err)
	}
	if err = copyTree(dst, ".", src, true); err != nil {
		return fmt.Errorf("unable to copy root directory to /app: %v", err)
	}
	return nil
}

// stageFlex stages a GAE Flex app. Does not support modules.
func stageFlex(src, dst string, buildCtx *build.Context) error {
	skippedPackages["appengine"] = false // Only exists for legacy App Engine Standard

	mainPathFile := filepath.Join(dst, "_gopath", "main-package-path")
	if err := writeMainPkgFile(mainPathFile, src); err != nil {
		return fmt.Errorf("failed to write %s: %v", mainPathFile, err)
	}
	// Find all dependencies for a build.Context for the release version and bundle their
	// directories into the staged directory.
	deps, err := analyze(src, buildCtx, true /* enforceMain */)
	if err != nil {
		return fmt.Errorf("failed analyzing %s: %v\nGOPATH: %s", src, err, buildCtx.GOPATH)
	}
	if err = bundle(dst, filepath.Join("_gopath", "src"), deps); err != nil {
		return fmt.Errorf("failed to bundle to %s: %v", dst, err)
	}
	if err = copyTree(dst, ".", src, true); err != nil {
		return fmt.Errorf("unable to copy root directory to /app: %v", err)
	}
	return nil
}

// stageStandardSecondGen stages an App Engine Standard second-gen app. Supports both vendoring and modules.
func stageStandardSecondGen(src, dst string, buildCtx *build.Context) error {
	skippedPackages["appengine"] = false // Only exists for legacy App Engine Standard

	gmPath, err := goModPath(src)
	if err != nil {
		log.Fatalf("failed finding go.mod: %v\n", err)
	}
	go111module := strings.ToLower(os.Getenv("GO111MODULE"))
	// Go 1.11 has the following logic for GO111MODULE:
	// If app is not on the GOPATH, use vgo
	// Else if app *IS* on the GOPATH and has go.mod:
	//   if GO111MODULE=on, use new vgo behavior
	//   else use old GOPATH behavior
	if gmPath == "" || (go111module != "on" && filepath.HasPrefix(gmPath, build.Default.GOPATH)) {
		fmt.Println("building with dependencies from GOPATH")
		return stageFlex(src, dst, buildCtx)
	}
	fmt.Println("building with dependencies from go.mod")

	// If a go.mod file was found, we assume all dependencies are either local
	// to the module directory or will be fetched by the builder, so we don't
	// need to walk the local filesystem or analyze imports.
	mainPathFile := filepath.Join(dst, "_main-package-path")
	if err := writeGoModMainPkgFile(mainPathFile, gmPath, src); err != nil {
		return fmt.Errorf("failed to write %s: %v", mainPathFile, err)
	}
	srcRoot := filepath.Dir(gmPath)

	// TODO Make sure this follows symlinks
	if err = copyTree(dst, ".", srcRoot, true); err != nil {
		return fmt.Errorf("unable to copy root directory to /app: %v", err)
	}
	return nil
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

func writeGoModMainPkgFile(dst, goModPath, appDir string) error {
	mainPath, err := goModRelativeBuildPath(goModPath, appDir)
	if err != nil {
		return fmt.Errorf("could not find relative app path: %v", err)
	}
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("unable to create directory %q: %v", dstDir, err)
	}
	// Write out mainPath to file.
	f, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("unable to create %q: %v", dst, err)
	}
	if _, err := f.WriteString(mainPath); err != nil {
		return fmt.Errorf("unable to write %q: %v", dst, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("unable to close %q: %v", dst, err)
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
	if cfg.isLegacyStandard() {
		return stdMinorVersion(cfg)
	} else if cfg.isFlex() {
		return flexMinorVersion(cfg)
	}
	return secondGenMinorVersion(cfg)
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

// secondGenMinorVersion returns minor version for GAE Standard 2nd gen.
func secondGenMinorVersion(cfg *config) (int, error) {
	runtime := cfg.Runtime
	runtime = strings.Replace(runtime, "go1", "go1.", 1)
	mv, ok := parseGo1MinorVersion(runtime)
	if !ok {
		// Invalid value.
		return -1, fmt.Errorf("invalid runtime value %s", cfg.Runtime)
	}
	return mv, nil
}

const bugReportMsg = `This may be a bug, please file a report at https://issuetracker.google.com/issues/new?component=322870.`

// flexMinorVersion returns minor version for GAE Flex.
func flexMinorVersion(cfg *config) (int, error) {
	val := cfg.Runtime
	if val == "go" {
		// Error coming from determining the default version may be a bug in the publishing
		// system/process.  Add statement to error message on how to report bug.
		mv, err := flexDefaultMinorVersion()
		if err != nil {
			return 0, fmt.Errorf("%v\n%s", err, bugReportMsg)
		}
		return mv, nil
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

// flexDefaultMinorVersion returns the default minor version for Flex based on
// configuration in flexRuntimesConfigURL.
func flexDefaultMinorVersion() (int, error) {
	type runtimesConfig struct {
		Runtimes map[string]struct {
			Target struct {
				Runtime string
			}
		}
	}

	b, err := readFlexRuntimesConfig()
	if err != nil {
		return 0, err
	}
	cfg := &runtimesConfig{}
	if err := yaml.Unmarshal(b, cfg); err != nil {
		return 0, fmt.Errorf("failed to parse runtimes.yaml: %v", err)
	}
	rt, ok := cfg.Runtimes["go"]
	if !ok {
		return 0, fmt.Errorf("missing go runtime config in runtimes.yaml")
	}
	target := rt.Target.Runtime
	if !strings.HasPrefix(target, "go1.") {
		return 0, fmt.Errorf("invalid go runtime version in runtimes.yaml: %s", target)
	}
	s := strings.TrimPrefix(target, "go1.")
	mv, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid go runtime version in runtimes.yaml: %s", target)
	}
	return mv, nil
}

func readFlexRuntimesConfig() ([]byte, error) {
	resp, err := http.Get(*flexRuntimesConfigURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download runtimes.yaml: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching runtimes.yaml returned status %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read runtimes.yaml: %v", err)
	}
	return body, nil
}

// buildContext returns the context for building the source.
func buildContext(tags []string, minorVersion int) *build.Context {
	var rels []string
	for i := 1; i <= minorVersion; i++ {
		rels = append(rels, fmt.Sprintf("go1.%d", i))
	}
	return &build.Context{
		GOARCH:      "amd64",
		GOOS:        "linux",
		GOROOT:      "",
		GOPATH:      build.Default.GOPATH,
		Compiler:    build.Default.Compiler,
		BuildTags:   tags,
		ReleaseTags: rels,
	}
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

// goModPath searches up the directory tree for a go.mod file, stopping at the
// first match and returning the path to the go.mod file. If no go.mod file is
// found, returns an empty string.
func goModPath(src string) (string, error) {
	src, err := filepath.Abs(src)
	if err != nil {
		return "", fmt.Errorf("src abspath: %v", err)
	}
	for {
		p := filepath.Join(src, "go.mod")
		_, err := os.Stat(p)
		if err == nil {
			return p, nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("unexpected error: %v", err)
		}
		oldSrc := src
		src = filepath.Dir(src)
		if oldSrc == src {
			break
		}
	}
	return "", nil
}

// unixSeparators converts non-unix \ path separators to unix / separators.
func unixSeparators(path string) string {
	return strings.Replace(path, `\`, `/`, -1)
}

// goModRelativeBuildPath returns the relative path to the package being built,
// given the root at the path to go.mod. This will return paths that look like
// "." or "foo" or "foo/bar"
func goModRelativeBuildPath(goModPath, appDir string) (string, error) {
	rootDir, err := filepath.Abs(filepath.Dir(goModPath))
	if err != nil {
		return "", fmt.Errorf("could not get absolute path for go.mod path %q: %v", goModPath, err)
	}
	mainDir, err := filepath.Abs(appDir)
	if err != nil {
		return "", fmt.Errorf("could not get absolute path for dir %q: %v", appDir, err)
	}
	if !strings.HasPrefix(mainDir, rootDir) {
		return "", fmt.Errorf("expected path '%q' to have prefix '%q'", mainDir, rootDir)
	}
	appPath, err := filepath.Rel(rootDir, mainDir)
	if err != nil {
		return "", fmt.Errorf("could not get relative path: %v", err)
	}
	return unixSeparators(appPath), nil
}

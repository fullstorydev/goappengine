// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/scanner"
	"go/token"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// App represents an entire Go App Engine app.
type App struct {
	Files        []*File    // the complete set of source files for this app
	Packages     []*Package // the packages
	RootPackages []*Package // the subset of packages with init functions
	HasMain      bool       // whether the app has a user-defined main package

	PackageIndex map[string]*Package // index from import path to package object
}

// Package represents a Go package.
type Package struct {
	ImportPath   string     // the path under which this package may be imported
	Files        []*File    // the set of source files that form this package
	BaseDir      string     // what the file names are relative to, if outside app
	SrcDir       string     // the source location of the files, always set
	Dependencies []*Package // the packages that this directly depends upon, in no particular order
	HasInit      bool       // whether the package has any init functions
	IsMain       bool       // whether the package is main
	Dupe         bool       // whether the package is a duplicate
	Synthetic    bool       // whether the package is a synthetic main or import tree package

	compiled chan struct{} // closed when the package has finished compiling
	hash     []byte        // a hash representing the package's sources and deps
}

func (p *Package) String() string {
	return fmt.Sprintf("%+v", *p)
}

// Implement sort.Interface for []*Package.
type byImportPath []*Package

func (s byImportPath) Len() int           { return len(s) }
func (s byImportPath) Less(i, j int) bool { return s[i].ImportPath < s[j].ImportPath }
func (s byImportPath) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type File struct {
	Name        string   // the file name
	PackageName string   // the package this file declares itself to be
	ImportPaths []string // import paths
	HasInit     bool     // whether the file has an init function
	HasMain     bool     // whether the file defines a main.main function
	callsAEMain bool     // whether the main function, if it exists, calls appengine.Main
}

func (f *File) String() string {
	return fmt.Sprintf("%+v", *f)
}

// Implement sort.Interface for []*File.
type byFileName []*File

func (s byFileName) Len() int           { return len(s) }
func (s byFileName) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s byFileName) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// vfs is a tiny VFS overlay that exposes a subset of files in a tree.
type vfs struct {
	baseDir   string
	filenames []string
}

func (v vfs) readDir(dir string) (fis []os.FileInfo, err error) {
	dir = filepath.Clean(dir)
	for _, f := range v.filenames {
		f = filepath.Join(v.baseDir, f)
		if filepath.Dir(f) == dir {
			fi, err := os.Stat(f)
			if err != nil {
				return nil, err
			}
			fis = append(fis, fi)
		}
	}
	return fis, nil
}

func buildContext(goPath string) *build.Context {
	ctxt := &build.Context{
		GOARCH:      build.Default.GOARCH,
		GOOS:        build.Default.GOOS,
		GOROOT:      *goRoot,
		GOPATH:      goPath,
		Compiler:    "gc",
		BuildTags:   []string{"appengine", "purego", "dev_appserver"},
		ReleaseTags: releaseTags(*apiVersion),
	}
	return ctxt
}

// ParseFiles parses the named files, deduces their package structure,
// and returns the dependency DAG as an App.
// Elements of filenames are considered relative to baseDir.
// If ignoreReleaseTags is true, ParseFiles will include files (and their
// dependencies) which would have been excluded because they included future
// release tags.
//
// TODO: Add a check that applications based outside gopath do
// not look like they're using vendoring: it doesn't work, and we don't want
// to add confusion.
func ParseFiles(baseDir string, filenames []string, ignoreReleaseTags bool) (*App, error) {
	app := &App{
		PackageIndex: make(map[string]*Package),
	}
	pkgFiles := make(map[string][]*File) // app package name => its files

	vfs := vfs{baseDir, filenames}

	ctxt := buildContext(baseDir)
	ctxt.HasSubdir = func(root, dir string) (rel string, ok bool) {
		// Override the default HasSubdir, which evaluates symlinks.
		const sep = string(filepath.Separator)
		root = filepath.Clean(root)
		if !strings.HasSuffix(root, sep) {
			root += sep
		}
		dir = filepath.Clean(dir)
		if !strings.HasPrefix(dir, root) {
			return "", false
		}
		return dir[len(root):], true
	}
	ctxt.ReadDir = func(dir string) ([]os.FileInfo, error) {
		return vfs.readDir(dir)
	}

	dirs := make(map[string]bool)
	for _, f := range filenames {
		dir := filepath.Dir(f) // "." for top-level files
		if dir == "" || dir == string(filepath.Separator) {
			return nil, fmt.Errorf("bad filename %q", f)
		}
		dirs[dir] = true
	}
	for dir := range dirs {
		pkg, err := ctxt.ImportDir(filepath.Join(baseDir, dir), 0)
		if _, ok := err.(*build.NoGoError); ok {
			// There were .go files, but they were all excluded (e.g. by "// +build ignore").
			continue
		}
		if err, ok := err.(*build.MultiplePackageError); ok {
			// Sanitize path.
			err.Dir = rel(baseDir, err.Dir)
		}
		if err != nil {
			return nil, fmt.Errorf("failed parsing dir %v: %v", dir, err)
		}

		for _, f := range pkg.GoFiles {
			filename := filepath.Join(dir, f)
			file, err := parseFile(baseDir, filename)
			if err != nil {
				return nil, err
			}
			app.Files = append(app.Files, file)
			pkgFiles[dir] = append(pkgFiles[dir], file)

			// Skip any main.main that isn't at the application root.
			if file.HasMain && dir == "." {
				app.HasMain = true
			}
		}

	}

	allowedDupes := make(map[string]bool)
	if *pkgDupes != "" {
		for _, pkg := range strings.Split(*pkgDupes, ",") {
			allowedDupes[pkg] = true
		}
	}

	// Create Package objects.
	for dirname, files := range pkgFiles {
		imp := filepath.ToSlash(dirname)
		if dirname == "." {
			if app.HasMain {
				imp = "main"
			} else {
				// top-level package; generate random package name for main.
				rng := rand.New(rand.NewSource(time.Now().Unix()))
				imp = fmt.Sprintf("main%05d", rng.Intn(1e5))
			}
		}

		p := &Package{
			ImportPath: imp,
			Files:      files,
			SrcDir:     filepath.Join(baseDir, dirname),
		}
		if isStandardPackage(p.ImportPath) {
			if !allowedDupes[p.ImportPath] {
				return nil, fmt.Errorf("package %q has the same name as a standard package", p.ImportPath)
			}
			p.Dupe = true
		}
		for _, f := range files {
			if f.HasMain && filepath.Dir(f.Name) == "." {
				p.IsMain = true
			}
			if f.HasInit {
				p.HasInit = true
			}
		}
		app.Packages = append(app.Packages, p)
		app.PackageIndex[p.ImportPath] = p
		if p.HasInit {
			app.RootPackages = append(app.RootPackages, p)
		}

		if p.IsMain {
			for _, f := range files {
				if f.HasMain && !f.callsAEMain {
					return nil, fmt.Errorf("parser: found a top level package main with function main, " +
						"but main does not call appengine.Main(); " +
						"see https://godoc.org/google.golang.org/appengine#Main for more information")
				}
			}
		}
	}

	if *goPath != "" {
		var re *regexp.Regexp
		var err error
		if *noBuildFiles != "" {
			re, err = regexp.Compile(*noBuildFiles)
			if err != nil {
				return nil, fmt.Errorf("bad -nobuild_files: %v", err)
			}
		}
		fs := appFilesInGOPATH(baseDir, *goPath, app)
		if err := addFromGOPATH(app, re, fs, ignoreReleaseTags); err != nil {
			return nil, err
		}
	}

	// Populate dependency lists.
	for _, p := range app.Packages {
		imports := make(map[string]int) // ImportPath => 1
		for _, f := range p.Files {
			for _, path := range f.ImportPaths {
				imports[path] = 1
			}
		}
		p.Dependencies = make([]*Package, 0, len(imports))
		for path := range imports {
			pkg, ok := app.PackageIndex[path]
			if !ok {
				// A file declared an import we don't know.
				// It could be a package from the standard library.
				if findInternal(path) {
					return nil, fmt.Errorf("package %q cannot import internal package %q", p.ImportPath, path)
				}
				continue
			}
			p.Dependencies = append(p.Dependencies, pkg)
		}
		sort.Sort(byImportPath(p.Dependencies))
	}

	// Sort topologically.
	if err := topologicalSort(app.Packages); err != nil {
		return nil, err
	}

	return app, nil
}

// appFilesInGOPATH returns a set of app files that are in the GOPATH.
// The constructed set of filenames is relative to the GOPATH's 'src' dir.
// If any of these files appear in a package's source files, an error
// is generated and the build fails.
func appFilesInGOPATH(baseDir, goPath string, app *App) map[string]bool {
	var gopathBase string
	for _, p := range filepath.SplitList(goPath) {
		prefix := filepath.Join(p, "src") + string(filepath.Separator)
		if strings.HasPrefix(baseDir, prefix) {
			gopathBase = baseDir[len(prefix):] // GOPATH-relative base of app's files
			break
		}
	}
	if gopathBase == "" {
		return nil // app not in a GOPATH
	}

	r := make(map[string]bool)
	for _, f := range app.Files {
		r[filepath.Join(gopathBase, f.Name)] = true
	}
	return r
}

func validatePkgPaths(pkg *build.Package, appFilesInGOPATH map[string]bool) error {
	for _, f := range pkg.GoFiles {
		n := filepath.Join(pkg.ImportPath, f)
		if _, ok := appFilesInGOPATH[n]; ok {
			return fmt.Errorf("app file %s conflicts with same file imported from GOPATH", f)
		}
	}
	return nil
}

// addFromGOPATH adds packages from GOPATH that are needed by the app.
func addFromGOPATH(app *App, noBuild *regexp.Regexp, appFilesInGOPATH map[string]bool, ignoreReleaseTags bool) error {
	warned := make(map[string]bool)
	for i := 0; i < len(app.Packages); i++ { // app.Packages is grown during this loop
		p := app.Packages[i]
		for _, f := range p.Files {
			for _, path := range f.ImportPaths {
				// Check for invalid imports.
				if !checkImport(path) {
					return fmt.Errorf("parser: bad import %q in %s from GOPATH", path, filepath.Join(p.ImportPath, f.Name))
				}
				if isStandardPackage(path) {
					continue
				}
				pkg, err := gopathPackage(path, p.SrcDir)
				if err != nil {
					if _, ok := app.PackageIndex[path]; ok {
						continue // Don't warn for packages we've seen before.
					}
					if !warned[path] {
						log.Printf("Can't find package %q in $GOPATH: %v", path, err)
						warned[path] = true
					}
					continue
				}
				// Check for duplicate imports.
				if p, ok := app.PackageIndex[path]; ok {
					if p.SrcDir == pkg.Dir {
						continue
					}
					return fmt.Errorf("package %q is imported from multiple locations: %q and %q", path, p.SrcDir, pkg.Dir)
				}
				// If requested, expand the package to include ignored release tags.
				if ignoreReleaseTags {
					if err := expandIgnoringReleases(pkg); err != nil {
						return err
					}
				}
				// Check the package doesn't use files already in the app.
				if err := validatePkgPaths(pkg, appFilesInGOPATH); err != nil {
					return err
				}

				files := make([]*File, 0, len(pkg.GoFiles))
				for _, f := range pkg.GoFiles {
					if noBuild != nil && noBuild.MatchString(filepath.Join(path, f)) {
						continue
					}
					files = append(files, &File{
						Name:        f,
						PackageName: pkg.Name,
						// NOTE: This is inaccurate, but it is sufficient to
						// record all the package imports for each file.
						ImportPaths: pkg.Imports,
					})
				}
				if len(files) == 0 {
					return fmt.Errorf("package %s required, but all its files were excluded by nobuild_files", path)
				}
				p := &Package{
					ImportPath: path,
					Files:      files,
					BaseDir:    pkg.Dir,
					SrcDir:     pkg.Dir,
				}
				app.Packages = append(app.Packages, p)
				app.PackageIndex[path] = p
			}
		}
	}
	return nil
}

// isInit returns whether the given function declaration is a true init function.
// Such a function must be called "init", not have a receiver, and have no arguments or return types.
func isInit(f *ast.FuncDecl) bool { return isNiladic(f, "init") }

// isMain returns whether the given function declaration is a main function.
// Such a function must be called "main", not have a receiver, and have no arguments or return types.
func isMain(f *ast.FuncDecl) bool { return isNiladic(f, "main") }

// callsAppEngineMain returns whether or not the given function calls
// appengine.Main(). This is required in user-provided main() functions.
func callsAppEngineMain(fset *token.FileSet, f *ast.FuncDecl) bool {
	for _, expr := range f.Body.List {
		expStmt, ok := expr.(*ast.ExprStmt)
		if !ok {
			continue
		}
		callExpr, ok := expStmt.X.(*ast.CallExpr)
		if !ok {
			continue
		}
		selExpr, ok := callExpr.Fun.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		selX, ok := selExpr.X.(*ast.Ident)
		if !ok {
			continue
		}
		if selX.Name == "appengine" && selExpr.Sel.Name == "Main" {
			return true
		}
	}
	return false
}

func isNiladic(f *ast.FuncDecl, name string) bool {
	ft := f.Type
	return f.Name.Name == name && f.Recv == nil && ft.Params.NumFields() == 0 && ft.Results.NumFields() == 0
}

func readFile(baseDir, filename string) (file *ast.File, fset *token.FileSet, err error) {
	fullName := filepath.Join(baseDir, filename)
	var src []byte
	src, err = ioutil.ReadFile(fullName)
	if err != nil {
		return
	}
	fset = token.NewFileSet()
	file, err = parser.ParseFile(fset, fullName, src, 0)
	return
}

// parseFile parses a single Go source file into a *File.
func parseFile(baseDir, filename string) (*File, error) {
	file, fset, err := readFile(baseDir, filename)
	if err != nil {
		return nil, err
	}

	// Walk the file's declarations looking for all the imports.
	// Determine whether the file has an init function at the same time.
	var imports []string
	hasInit := false
	hasMain := false
	callsAEMain := false
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
			for _, spec := range genDecl.Specs {
				importSpec := spec.(*ast.ImportSpec)
				val := string(importSpec.Path.Value)
				path, err := strconv.Unquote(val)
				if err != nil {
					return nil, fmt.Errorf("parser: bad ImportSpec %q: %v", val, err)
				}
				if !checkImport(path) {
					return nil, fmt.Errorf("parser: bad import %q in %s", path, filename)
				}
				imports = append(imports, path)
			}
		}
		if funcDecl, ok := decl.(*ast.FuncDecl); ok {
			if isInit(funcDecl) {
				hasInit = true
			}
			if file.Name.Name == "main" && isMain(funcDecl) {
				hasMain = true
				callsAEMain = callsAppEngineMain(fset, funcDecl)
			}
		}
	}

	// Check for unkeyed struct literals from the standard package library.
	ch := newCompLitChecker(fset)
	ast.Walk(ch, file)
	if len(ch.errors) > 0 {
		return nil, ch.errors
	}

	return &File{
		Name:        filename,
		PackageName: file.Name.Name,
		ImportPaths: imports,
		HasInit:     hasInit,
		HasMain:     hasMain,
		callsAEMain: callsAEMain,
	}, nil
}

var legalImportPath = regexp.MustCompile(`^[a-zA-Z0-9_\-./~+]+$`)

// checkImport will return whether the provided import path is good.
func checkImport(path string) bool {
	if path == "" {
		return false
	}
	if len(path) > 1024 {
		return false
	}
	if filepath.IsAbs(path) || strings.Contains(path, "..") {
		return false
	}
	if !legalImportPath.MatchString(path) {
		return false
	}
	if path == "syscall" || path == "unsafe" {
		return false
	}
	return true
}

type compLitChecker struct {
	fset    *token.FileSet
	imports map[string]string // Local name => import path; only standard packages.
	errors  scanner.ErrorList // accumulated errors
}

func newCompLitChecker(fset *token.FileSet) *compLitChecker {
	return &compLitChecker{
		fset:    fset,
		imports: make(map[string]string),
	}
}

func (c *compLitChecker) errorf(node ast.Node, format string, a ...interface{}) {
	c.errors = append(c.errors, &scanner.Error{
		Pos: c.fset.Position(node.Pos()),
		Msg: fmt.Sprintf(format, a...),
	})
}

func (c *compLitChecker) Visit(node ast.Node) ast.Visitor {
	if imp, ok := node.(*ast.ImportSpec); ok {
		pth, _ := strconv.Unquote(imp.Path.Value)
		if !isStandardPackage(pth) {
			return c
		}
		if imp.Name != nil {
			id := imp.Name.Name
			if id == "." {
				return c
			}
			c.imports[id] = pth
		} else {
			// All standard packages have their last path component as their package name.
			c.imports[filepath.Base(pth)] = pth
		}
		return c
	}

	lit, ok := node.(*ast.CompositeLit)
	if !ok {
		return c
	}
	sel, ok := lit.Type.(*ast.SelectorExpr)
	if !ok {
		return c
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return c
	}
	pth, ok := c.imports[id.Name]
	if !ok {
		// This must be pkg.T for a package in the app.
		return c
	}

	// Check exception list.
	if unkeyedLiteral[pth+"."+sel.Sel.Name] {
		return c
	}

	allKeys := true
	for _, elt := range lit.Elts {
		_, ok := elt.(*ast.KeyValueExpr)
		allKeys = allKeys && ok
	}
	if !allKeys {
		c.errorf(lit, "composite struct literal %v.%v with unkeyed fields", pth, sel.Sel)
	}

	return c
}

// Cache of standard package status.
var stdPackageCache = map[string]bool{
	// There's no unsafe.a, but "unsafe" is a standard package.
	// Mention it explicitly here so we avoid a useless warning.
	"unsafe": true,
}

// isStandardPackage reports whether import path s is a standard package.
func isStandardPackage(s string) bool {
	if std, ok := stdPackageCache[s]; ok {
		return std
	}

	// Don't consider any import path containing a dot to be a standard package.
	if strings.Contains(s, ".") {
		stdPackageCache[s] = false
		return false
	}

	ctxt := buildContext("")
	pkg, err := ctxt.Import(s, "/nowhere", build.FindOnly|build.AllowBinary)
	if err != nil {
		stdPackageCache[s] = false
		return false
	}
	std := pkg.ImportPath != ""
	stdPackageCache[s] = std
	return std
}

// gopathPackage imports information about a package in GOPATH.
func gopathPackage(s, srcDir string) (*build.Package, error) {
	ctxt := buildContext(*goPath)
	// Don't use FindOnly or AllowBinary because we want import information
	// and we require the source files.
	return ctxt.Import(s, srcDir, 0)
}

// findReleaseTagSets returns the set of release tags which might influence the
// files chosen in the given package for future releases.
func findReleaseTagsSet(pkg *build.Package) [][]string {
	if len(pkg.IgnoredGoFiles) == 0 {
		return nil // No files were ignored.
	}

	// minVersion is the smallest Go version we expect this application to
	// build against (ie. the oldest Go version available in production).
	// It's okay if it falls out of date: at worst we do more work and upload
	// more files than necessary.
	const minVersion = 4

	// Find the all the go1.x tags which affected file selection
	// (where x > minVersion).
	const prefix = "go1."
	vs := make(map[int]bool)
	maxV := 0
	for _, tag := range pkg.AllTags {
		if !strings.HasPrefix(tag, prefix) {
			continue
		}
		v, err := strconv.Atoi(tag[len(prefix):])
		if err == nil && v > minVersion {
			vs[v] = true
			if v > maxV {
				maxV = v
			}
		}
	}
	if maxV == 0 {
		return nil
	}

	// Generate the sequence "go1.1", "go1.2", etc.
	tags := make([]string, 0, maxV)
	for i := 1; i <= maxV; i++ {
		tags = append(tags, fmt.Sprintf("go1.%d", i))
	}

	// Generate the set of all relevant release tags.
	allTags := [][]string{
		tags[:minVersion], // Always add the minVersion as a baseline.
	}
	for v := range vs {
		allTags = append(allTags, tags[:v])
	}
	return allTags
}

// expandIgnoringReleases expands the list of Go files and imports for the
// given package by considering all relevant sets of release tags to satisfy
// all possible future releases.
func expandIgnoringReleases(pkg *build.Package) error {
	tagsSet := findReleaseTagsSet(pkg)
	if len(tagsSet) == 0 {
		return nil
	}

	goFiles := make(map[string]bool, len(pkg.GoFiles))
	imports := make(map[string]bool, len(pkg.Imports))
	addToSet(goFiles, pkg.GoFiles)
	addToSet(imports, pkg.Imports)

	ctxt := buildContext(*goPath)
	for _, tags := range tagsSet {
		// NOTE: we could use ctxt.MatchFile against
		// pkg.IgnoredGoFiles to check that this set of tags will have
		// some effect before doing a full ImportDir if this approach
		// proves to be too slow.
		ctxt.ReleaseTags = tags

		// Don't use FindOnly or AllowBinary because we want import information
		// and we require the source files.
		p, err := ctxt.ImportDir(pkg.Dir, 0)
		if err != nil {
			return err
		}
		addToSet(goFiles, p.GoFiles)
		addToSet(imports, p.Imports)
	}

	pkg.GoFiles = pkg.GoFiles[:0]
	for x := range goFiles {
		pkg.GoFiles = append(pkg.GoFiles, x)
	}
	sort.Strings(pkg.GoFiles)
	pkg.Imports = pkg.Imports[:0]
	for x := range imports {
		pkg.Imports = append(pkg.Imports, x)
	}
	sort.Strings(pkg.Imports)

	return nil
}

// addToSet adds the strings from x to set.
func addToSet(set map[string]bool, x []string) {
	for _, s := range x {
		set[s] = true
	}
}

// topologicalSort sorts the given slice of *Package in topological order.
// The ordering is such that X comes before Y if X is a dependency of Y.
// A cyclic dependency graph is signalled by an error being returned.
func topologicalSort(p []*Package) error {
	selected := make(map[*Package]bool, len(p))
	for len(p) > 0 {
		// Sweep the working list and move the packages with no
		// selected dependencies to the front.
		//
		// n acts as both a count of the dependency-free packages,
		// and as the marker for the position of the first package
		// with a dependency that can be swapped to a later position.
		n := 0
	packageLoop:
		for i, pkg := range p {
			for _, dep := range pkg.Dependencies {
				if !selected[dep] {
					continue packageLoop
				}
			}
			selected[pkg] = true
			p[i], p[n] = p[n], pkg
			n++
		}
		if n == 0 {
			// No leaves, so there must be a cycle.
			cycle := findCycle(p)
			paths := make([]string, len(cycle)+1)
			for i, pkg := range cycle {
				paths[i] = pkg.ImportPath
			}
			paths[len(cycle)] = cycle[0].ImportPath // duplicate last package
			return fmt.Errorf("parser: cyclic dependency graph: %s", strings.Join(paths, " -> "))
		}
		p = p[n:]
	}
	return nil
}

// findCycle finds a cycle in pkgs.
// It assumes that a cycle exists.
func findCycle(pkgs []*Package) []*Package {
	pkgMap := make(map[*Package]bool, len(pkgs)) // quick index of packages
	var min *Package
	for _, pkg := range pkgs {
		pkgMap[pkg] = true
		if min == nil || pkg.ImportPath < min.ImportPath {
			min = pkg
		}
	}

	// Every element of pkgs is a member of a cycle,
	// so find a cycle starting with the first one lexically.
	cycle := []*Package{min}
	seen := map[*Package]int{min: 0} // map of package to index in cycle
	for {
		last := cycle[len(cycle)-1]
		for _, dep := range last.Dependencies {
			if i, ok := seen[dep]; ok {
				// Cycle found.
				return cycle[i:]
			}
		}
		// None of the dependencies of last are in cycle, so pick one of
		// its dependencies (that we know is in a cycle) to add to cycle.
		// We are always able to find such a dependency, because
		// otherwise last would not be a member of a cycle.
		var dep *Package
		for _, d := range last.Dependencies {
			if pkgMap[d] {
				dep = d
				break
			}
		}

		seen[dep] = len(cycle)
		cycle = append(cycle, dep)
	}
}

// findInternal returns whether the pkg path contains an "internal" path element.
func findInternal(path string) bool {
	return strings.HasSuffix(path, "/internal") ||
		strings.HasPrefix(path, "internal/") ||
		strings.Contains(path, "/internal/") ||
		path == "internal"
}

// constructRootPackageTree takes an unbounded-size list of root packages that
// need to be imported by the synthetic main package, and constructs a new list
// of root packages of size bounded by the given limit, such that importing
// those packages will transitively import all the input root packages.  This
// reduces the problem of a single compilation having a very large number of
// direct imports.
//
// Constructs a tree of new synthetic packages as necessary, such that none of
// those packages import more than the given limit of packages.  Source files
// are created for them.
//
// For example, with limit=2 and 5 root packages, it changes this:
//
// main->[a, b, c, d, e]
//
// to this:
//
// t1->[a, b], t2->[c, d], t3->[e, t1], main->[t2, t3]
//
// It returns a slice of the additional packages created, and a new slice of the
// root packages that the main package should import (which could include some
// packages from the original list in rootPackages.)
func constructRootPackageTree(rootPackages []*Package, limit int) (newPackages []*Package, newRootPackages []*Package, err error) {
	var (
		files []string
		count int
	)
	defer func() {
		if err != nil {
			for _, f := range files {
				os.Remove(f)
			}
		}
	}()
	newRootPackages = make([]*Package, len(rootPackages))
	copy(newRootPackages, rootPackages)
	for len(newRootPackages) > limit {
		// Modify newPackages and newRootPackages to add an additional tree node package.
		count++
		packageName := fmt.Sprintf("_import_tree%d", count)
		dir := filepath.Join(*workDir, packageName)
		filename := fmt.Sprintf("_go_main_tree%d.go", count)
		filePath := filepath.Join(dir, filename)
		file := &File{
			Name:        filePath,
			PackageName: packageName,
		}
		p := &Package{
			ImportPath: packageName,
			Files:      []*File{file},
			Synthetic:  true,
		}
		newPackages = append(newPackages, p)
		p.Dependencies, newRootPackages = newRootPackages[0:limit], append(newRootPackages[limit:], p)

		// Write the source file for the new package.
		var depPackageNames []string
		for _, d := range p.Dependencies {
			depPackageNames = append(depPackageNames, d.ImportPath)
		}
		nodeStr, err := MakeExtraImports(packageName, depPackageNames)
		if err != nil {
			return nil, nil, err
		}
		if err = os.MkdirAll(dir, 0750); err != nil {
			return nil, nil, err
		}
		files = append(files, filePath)
		if err = ioutil.WriteFile(filePath, []byte(nodeStr), 0640); err != nil {
			return nil, nil, err
		}
	}
	return newPackages, newRootPackages, nil
}

// Copied from golang @ src/cmd/vet/internal/whitelist/whitelist.go because
// go1.7 moved it to an internal package
var unkeyedLiteral = map[string]bool{
	// original things from whitelist package
	// These image and image/color struct types are frozen. We will never add fields to them.
	"image/color.Alpha16": true,
	"image/color.Alpha":   true,
	"image/color.CMYK":    true,
	"image/color.Gray16":  true,
	"image/color.Gray":    true,
	"image/color.NRGBA64": true,
	"image/color.NRGBA":   true,
	"image/color.NYCbCrA": true,
	"image/color.RGBA64":  true,
	"image/color.RGBA":    true,
	"image/color.YCbCr":   true,
	"image.Point":         true,
	"image.Rectangle":     true,
	"image.Uniform":       true,

	"unicode.Range16": true,

	// Our whitelisted stuff
	// Add some App Engine-specific entries to the unkeyed literal whitelist.
	"appengine/datastore.PropertyList": true,
	"appengine.MultiError":             true,

	// These were removed from go vet in
	// https://go-review.googlesource.com/#/c/22318/ so we must keep a copy here.
	"crypto/x509/pkix.RDNSequence":                  true,
	"crypto/x509/pkix.RelativeDistinguishedNameSET": true,
	"database/sql.RawBytes":                         true,
	"debug/macho.LoadBytes":                         true,
	"encoding/asn1.ObjectIdentifier":                true,
	"encoding/asn1.RawContent":                      true,
	"encoding/json.RawMessage":                      true,
	"encoding/xml.CharData":                         true,
	"encoding/xml.Comment":                          true,
	"encoding/xml.Directive":                        true,
	"go/scanner.ErrorList":                          true,
	"image/color.Palette":                           true,
	"net.HardwareAddr":                              true,
	"net.IP":                                        true,
	"net.IPMask":                                    true,
	"sort.Float64Slice":                             true,
	"sort.IntSlice":                                 true,
	"sort.StringSlice":                              true,
	"unicode.SpecialCase":                           true,
}

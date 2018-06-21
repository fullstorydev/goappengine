// Package deploy implements the `goapp deploy` command.
package deploy

import (
	"cmd/go/internal/base"
	"cmd/go/internal/load"
	"errors"
	"fmt"
	"go/build"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
)

// CmdDeploy is the configuration for the deploy command
var CmdDeploy = &base.Command{
	UsageLine: "deploy [deploy flags] [ application_dir | package | yaml_files...]",
	Short:     "deploys your application to App Engine",
	Long: `
Deploy uploads your application files to Google App Engine, and then compiles
and launches your application.

The argument to this command should be your application's root directory or a
single package which contains an app.yaml file. If you are using the Modules
feature, then you should pass multiple YAML files to deploy, rather than a
directory, to specify which modules to update. If no arguments are provided,
deploy looks in your current directory for an app.yaml file.

The -application flag sets the application ID, overriding the application value
from the app.yaml file.

The -version flag sets the major version, overriding the version value from the
app.yaml file.

The -oauth flag causes authentication to be done using OAuth2, instead of
interactive password auth.

This command wraps the appcfg.py command provided as part of the App Engine
SDK. For help using that command directly, run:
  ./appcfg.py help update
  `,
}

var (
	deployApp   string // deploy -application flag
	deployVer   string // deploy -version flag
	deployOAuth bool   // deploy -oauth flag
)

func init() {
	// break init cycle
	CmdDeploy.Run = runDeploy

	CmdDeploy.Flag.StringVar(&deployApp, "application", "", "")
	CmdDeploy.Flag.StringVar(&deployVer, "version", "", "")
	CmdDeploy.Flag.BoolVar(&deployOAuth, "oauth", false, "")
}

func runDeploy(cmd *base.Command, args []string) {
	appcfg, err := findAppcfg()
	if err != nil {
		base.Fatalf("goapp serve: %v", err)
	}
	toolArgs := []string{"update"}
	if deployApp != "" {
		toolArgs = append(toolArgs, "--application", deployApp)
	}
	if deployVer != "" {
		toolArgs = append(toolArgs, "--version", deployVer)
	}
	if deployOAuth {
		toolArgs = append(toolArgs, "--oauth2")
	}
	files, err := resolveAppFiles(args)
	if err != nil {
		base.Fatalf("goapp deploy: %v", err)
	}
	runSDKTool(appcfg, append(toolArgs, files...))
}

func findAppcfg() (string, error) {
	devAppserver, err := findDevAppserver()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(devAppserver), "appcfg.py"), nil
}

func runSDKTool(tool string, args []string) {
	python, err := findPython()
	if err != nil {
		base.Fatalf("could not find python interpreter: %v", err)
	}

	toolName := filepath.Base(tool)

	cmd := exec.Command(python, tool)
	cmd.Args = append(cmd.Args, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err = cmd.Start(); err != nil {
		base.Fatalf("error starting %s: %v", toolName, err)
	}

	// Swallow SIGINT. The tool will catch it and shut down cleanly.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		for s := range sig {
			log.Printf("goapp: caught SIGINT, waiting for %s to shut down", toolName)
			cmd.Process.Signal(s)
		}
	}()

	if err = cmd.Wait(); err != nil {
		base.Errorf("error while running %s: %v", toolName, err)
	}
}

func findPython() (path string, err error) {
	for _, name := range []string{"python2.7", "python"} {
		path, err = exec.LookPath(name)
		if err == nil {
			return
		}
	}
	return
}

func findDevAppserver() (string, error) {
	if p := os.Getenv("APPENGINE_DEV_APPSERVER"); p != "" {
		return p, nil
	}
	return "", fmt.Errorf("unable to find dev_appserver.py")
}

// resolveAppFiles returns a list of arguments suitable for passing appcfg.py
// or dev_appserver.py corresponding to the user-provided args.
func resolveAppFiles(args []string) ([]string, error) {
	if len(args) == 0 {
		if fileExists("app.yaml") {
			return []string{"./"}, nil
		}
		return nil, errors.New("no app.yaml file in current directory")
	}

	if len(args) == 1 && !strings.HasSuffix(args[0], ".yaml") {
		if fileExists(filepath.Join(args[0], "app.yaml")) {
			return args, nil
		}
		// Try to resolve this arg as a package.
		if build.IsLocalImport(args[0]) {
			return nil, fmt.Errorf("unable to find app.yaml at %s", args[0])
		}
		pkgs := load.Packages(args)
		if len(pkgs) > 1 {
			return nil, errors.New("only a single package may be provided")
		}
		if len(pkgs) == 0 {
			return nil, fmt.Errorf("unable to find app.yaml at %s (unable to resolve package)", args[0])
		}
		dir := pkgs[0].Dir
		if !fileExists(filepath.Join(dir, "app.yaml")) {
			return nil, fmt.Errorf("unable to find app.yaml at %s", dir)
		}
		return []string{dir}, nil
	}

	// The 1 or more args must all end with .yaml at this point.
	for _, a := range args {
		if !strings.HasSuffix(a, ".yaml") {
			return nil, fmt.Errorf("%s is not a YAML file", a)
		}
		if !fileExists(a) {
			return nil, fmt.Errorf("%s does not exist", a)
		}
	}
	return args, nil
}

func fileExists(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}

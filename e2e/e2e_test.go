// Package e2e exercises the compiled pkgwhen binary as a subprocess. Unit
// tests in the main package call run() directly; these tests build the
// actual artifact and verify exit codes and stdout/stderr routing through a
// real os.Exec boundary. They stay off the network.
package e2e

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binPath string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "pkgwhen-e2e-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "TestMain: mkdir:", err)
		os.Exit(2)
	}
	binPath = filepath.Join(tmp, "pkgwhen")
	out, buildErr := exec.Command("go", "build", "-o", binPath, "..").CombinedOutput()
	if buildErr != nil {
		fmt.Fprintf(os.Stderr, "TestMain: go build failed: %v\n%s", buildErr, out)
		os.RemoveAll(tmp)
		os.Exit(2)
	}
	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

type result struct {
	stdout   string
	stderr   string
	exitCode int
}

func runBin(t *testing.T, args ...string) result {
	t.Helper()
	return runBinWith(t, nil, args...)
}

// runBinWith runs the binary with extra environment entries, for the paths
// that need the process pointed somewhere.
func runBinWith(t *testing.T, extraEnv []string, args ...string) result {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Env = append(os.Environ(), extraEnv...)
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	if err == nil {
		return result{stdout: so.String(), stderr: se.String(), exitCode: 0}
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		t.Fatalf("run: %v", err)
	}
	return result{stdout: so.String(), stderr: se.String(), exitCode: ee.ExitCode()}
}

func TestHelp(t *testing.T) {
	r := runBin(t, "--help")
	if r.exitCode != 0 || !strings.HasPrefix(r.stdout, "pkgwhen — ") || r.stderr != "" {
		t.Errorf("--help = %+v", r)
	}
}

func TestVersion(t *testing.T) {
	r := runBin(t, "--version")
	if r.exitCode != 0 || strings.TrimSpace(r.stdout) == "" || r.stderr != "" {
		t.Errorf("--version = %+v", r)
	}
}

func TestInstructions(t *testing.T) {
	r := runBin(t, "--instructions")
	if r.exitCode != 0 || !strings.HasPrefix(r.stdout, "To find which versions") || strings.Count(r.stdout, "\n") != 1 {
		t.Errorf("--instructions = %+v", r)
	}
}

func TestUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no package", nil, "no package given"},
		{"bad registry", []string{"cargo:serde"}, "unknown registry"},
		{"ambiguous unit", []string{"--min-age", "1m", "pypi:x"}, "minutes or months"},
		{"bad limit", []string{"-n", "0", "pypi:x"}, "positive integer"},
		{"contradictory window", []string{"--min-age", "7d", "--since", "1d", "pypi:x"}, "nothing can match"},
		{"dead option with a version", []string{"--min-age", "1d", "pypi:x@1.2.3"}, "does not apply with @VERSION"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := runBin(t, tt.args...)
			if r.exitCode != 2 || r.stdout != "" || !strings.HasPrefix(r.stderr, "pkgwhen: ") || !strings.Contains(r.stderr, tt.want) {
				t.Errorf("%v = %+v", tt.args, r)
			}
		})
	}
}

// TestRegistryError points the process at a closed port, so the request
// fails in transport before any registry answers. That is the exit code for
// "could not be reached", which is what a waiting loop has to stop on.
func TestRegistryError(t *testing.T) {
	env := []string{"HTTP_PROXY=http://127.0.0.1:9", "HTTPS_PROXY=http://127.0.0.1:9", "NO_PROXY="}
	for _, args := range [][]string{{"pypi:openai-agents"}, {"pypi:openai-agents@0.22.0"}} {
		r := runBinWith(t, env, args...)
		if r.exitCode != 3 || r.stdout != "" || !strings.HasPrefix(r.stderr, "pkgwhen: ") {
			t.Errorf("%v = %+v", args, r)
		}
	}
}

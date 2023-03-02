package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

func buildTags() string {
	mode := strings.ToLower(os.Getenv("WASI_TEST_MODE"))

	var tags []string
	if mode == "cgo" {
		tags = append(tags, "libinjection_cgo")
	}

	return strings.Join(tags, ",")
}

// Test runs unit tests - by default, it uses wazero; set WASI_TEST_MODE=cgo or WASI_TEST_MODE=tinygo to use either
func Test() error {
	mode := strings.ToLower(os.Getenv("WASI_TEST_MODE"))

	if mode != "tinygo" {
		return sh.RunV("go", "test", "-v", "-timeout=20m", "-tags", buildTags(), "./...")
	}

	return sh.RunV("tinygo", "test", "-target=wasi", "-v", "-tags", buildTags(), "./...")
}

func Format() error {
	if err := sh.RunV("go", "run", fmt.Sprintf("mvdan.cc/gofumpt@%s", verGoFumpt), "-l", "-w", "."); err != nil {
		return err
	}
	if err := sh.RunV("go", "run", fmt.Sprintf("github.com/rinchsan/gosimports/cmd/gosimports@%s", verGosImports), "-w",
		"-local", "github.com/wasilibs/go-libinjection",
		"."); err != nil {
		return nil
	}
	return nil
}

func Lint() error {
	return sh.RunV("go", "run", fmt.Sprintf("github.com/golangci/golangci-lint/cmd/golangci-lint@%s", verGolangCILint), "run", "--build-tags", buildTags())
}

// Check runs lint and tests.
func Check() {
	mg.SerialDeps(Lint, Test)
}

// UpdateLibs updates the precompiled wasm libraries.
func UpdateLibs() error {
	if err := sh.RunV("docker", "build", "-t", "ghcr.io/wasilibs/go-libinjection/buildtools-libinjection", "-f", filepath.Join("buildtools", "libinjection", "Dockerfile"), "."); err != nil {
		return err
	}
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	return sh.RunV("docker", "run", "-it", "--rm", "-v", fmt.Sprintf("%s:/out", filepath.Join(wd, "wasm")), "ghcr.io/wasilibs/go-libinjection/buildtools-libinjection")
}

// Bench runs benchmarks in the default configuration for a Go app, using wazero.
func Bench() error {
	return sh.RunV("go", benchArgs("./...", 1, benchModeWazero)...)
}

// BenchCGO runs benchmarks with injection accessed using cgo. A C++ toolchain and libinjection must be installed to run.
func BenchCGO() error {
	return sh.RunV("go", benchArgs("./...", 1, benchModeCGO)...)
}

// BenchDefault runs benchmarks using the regexp library in the standard library for comparison.
func BenchDefault() error {
	return sh.RunV("go", benchArgs("./...", 1, benchModeDefault)...)
}

// BenchAll runs all benchmark types and outputs with benchstat. A C++ toolchain and libinjection must be installed to run.
func BenchAll() error {
	if err := os.MkdirAll("build", 0o755); err != nil {
		return err
	}

	fmt.Println("Executing wazero benchmarks")
	wazero, err := sh.Output("go", benchArgs("./...", 5, benchModeWazero)...)
	if err != nil {
		return fmt.Errorf("error running wazero benchmarks: %w", err)
	}
	if err := os.WriteFile(filepath.Join("build", "bench.txt"), []byte(wazero), 0o644); err != nil {
		return err
	}

	fmt.Println("Executing cgo benchmarks")
	cgo, err := sh.Output("go", benchArgs("./...", 5, benchModeCGO)...)
	if err != nil {
		fmt.Println("Error running cgo benchmarks:")
		return fmt.Errorf("error running cgo benchmarks: %w", err)
	}
	if err := os.WriteFile(filepath.Join("build", "bench_cgo.txt"), []byte(cgo), 0o644); err != nil {
		return err
	}

	fmt.Println("Executing default benchmarks")
	def, err := sh.Output("go", benchArgs("./...", 5, benchModeDefault)...)
	if err != nil {
		return fmt.Errorf("error running default benchmarks: %w", err)
	}
	if err := os.WriteFile(filepath.Join("build", "bench_default.txt"), []byte(def), 0o644); err != nil {
		return err
	}

	return sh.RunV("go", "run", fmt.Sprintf("golang.org/x/perf/cmd/benchstat@%s", verBenchstat),
		"build/bench_default.txt", "build/bench.txt", "build/bench_cgo.txt")
}

// WAFBench runs benchmarks in the default configuration for a Go app, using wazero.
func WAFBench() error {
	return sh.RunV("go", benchArgs("./wafbench", 1, benchModeWazero)...)
}

// WAFBenchCGO runs benchmarks with injection accessed using cgo. A C++ toolchain and libinjection must be installed to run.
func WAFBenchCGO() error {
	return sh.RunV("go", benchArgs("./wafbench", 1, benchModeCGO)...)
}

// WAFBenchDefault runs benchmarks using the regexp library in the standard library for comparison.
func WAFBenchDefault() error {
	return sh.RunV("go", benchArgs("./wafbench", 1, benchModeDefault)...)
}

// WAFBenchAll runs all benchmark types and outputs with benchstat. A C++ toolchain and libinjection must be installed to run.
func WAFBenchAll() error {
	if err := os.MkdirAll("build", 0o755); err != nil {
		return err
	}

	fmt.Println("Executing wazero benchmarks")
	wazero, err := sh.Output("go", benchArgs("./wafbench", 5, benchModeWazero)...)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join("build", "wafbench.txt"), []byte(wazero), 0o644); err != nil {
		return err
	}

	fmt.Println("Executing cgo benchmarks")
	cgo, err := sh.Output("go", benchArgs("./wafbench", 5, benchModeCGO)...)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join("build", "wafbench_cgo.txt"), []byte(cgo), 0o644); err != nil {
		return err
	}

	fmt.Println("Executing default benchmarks")
	def, err := sh.Output("go", benchArgs("./wafbench", 5, benchModeDefault)...)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join("build", "wafbench_default.txt"), []byte(def), 0o644); err != nil {
		return err
	}

	return sh.RunV("go", "run", fmt.Sprintf("golang.org/x/perf/cmd/benchstat@%s", verBenchstat),
		"build/wafbench_default.txt", "build/wafbench.txt", "build/wafbench_cgo.txt")
}

var Default = Test

type benchMode int

const (
	benchModeWazero benchMode = iota
	benchModeCGO
	benchModeDefault
)

func benchArgs(pkg string, count int, mode benchMode) []string {
	args := []string{"test", "-bench=.", "-run=^$", "-v", "-timeout=60m"}
	if count > 0 {
		args = append(args, fmt.Sprintf("-count=%d", count))
	}
	switch mode {
	case benchModeCGO:
		args = append(args, "-tags=libinjection_cgo")
	case benchModeDefault:
		args = append(args, "-tags=libinjection_bench_default")
	}
	args = append(args, pkg)

	return args
}

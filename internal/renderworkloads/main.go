// internal/renderworkloads/main.go
//
// Renders YAML templates (using gomplate) under a given input root, writing ONLY rendered
// collector workload YAMLs (Deployment/DaemonSet/StatefulSet) to an output directory while
// preserving relative paths.
// Also writes workloads.txt containing paths to rendered workload YAMLs.
//
// Values are provided via a JSON file (default: render-vars.json) located in -repo-root.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

const (
	defaultOutBase  = "/tmp/rendered"
	defaultGomplate = "gomplate"
	defaultVarsFile = "render-vars.json"

	collectorLabelKey   = "app.kubernetes.io/name"
	collectorLabelValue = "opentelemetry-collector"

	workloadsIndexName = "workloads.txt"
)

var workloadKinds = map[string]struct{}{
	"Deployment":  {},
	"DaemonSet":   {},
	"StatefulSet": {},
}

type Options struct {
	RepoRoot   string
	InRoot     string
	OutBase    string
	Gomplate   string
	VarsFile   string
	WriteIndex bool
	Verbose    bool
}

func main() {
	opt := parseFlags()

	if opt.RepoRoot == "" || opt.InRoot == "" {
		fatalf("error: -repo-root and -in-root are required\n")
	}

	repoRoot := mustAbs(opt.RepoRoot)

	inRoot := mustAbs(filepath.Join(repoRoot, opt.InRoot))
	if _, err := os.Stat(inRoot); err != nil {
		fatalf("error: input root does not exist: %s: %v\n", inRoot, err)
	}

	outBase := mustAbs(opt.OutBase)
	if err := os.MkdirAll(outBase, 0o755); err != nil {
		fatalf("error: cannot create out-base %s: %v\n", outBase, err)
	}

	varsPath := mustAbs(filepath.Join(repoRoot, opt.VarsFile))
	if _, err := os.Stat(varsPath); err != nil {
		fatalf("error: vars file not found: %s: %v\n", varsPath, err)
	}

	workloads, err := renderCollectorWorkloads(repoRoot, inRoot, outBase, varsPath, opt)
	if err != nil {
		// mirror the behavior you saw (panic-ish), but with a clearer message
		fatalf("panic: %v\n", err)
	}

	if opt.WriteIndex {
		indexPath := filepath.Join(outBase, workloadsIndexName)

		content := strings.Join(workloads, "\n")
		if len(content) > 0 {
			content += "\n"
		}

		if err := os.WriteFile(indexPath, []byte(content), 0o644); err != nil {
			fatalf("error: writing workloads index: %v\n", err)
		}
		fmt.Printf("Wrote workload index: %s\n", indexPath)
	}

	fmt.Printf("Rendered collector workloads from %s to %s\n", inRoot, outBase)
}

func parseFlags() Options {
	var opt Options
	flag.StringVar(&opt.RepoRoot, "repo-root", "", "Repository root (used to compute relative paths and locate vars file)")
	flag.StringVar(&opt.InRoot, "in-root", "", "Input root directory (relative to -repo-root) to scan for YAML templates")
	flag.StringVar(&opt.OutBase, "out-base", defaultOutBase, "Output base directory")
	flag.StringVar(&opt.Gomplate, "gomplate", defaultGomplate, "Path to gomplate binary")
	flag.StringVar(&opt.VarsFile, "vars-file", defaultVarsFile, "Vars JSON file name (resolved relative to -repo-root)")
	flag.BoolVar(&opt.WriteIndex, "write-index", true, "Write workloads.txt with rendered workload YAML paths")
	flag.BoolVar(&opt.Verbose, "verbose", false, "Verbose output (print gomplate commands)")
	flag.Parse()
	return opt
}

func renderCollectorWorkloads(repoRoot, inRoot, outBase, varsPath string, opt Options) ([]string, error) {
	workloads := make([]string, 0, 128)

	err := filepath.WalkDir(inRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			switch filepath.Base(path) {
			case ".git", "vendor":
				return filepath.SkipDir
			default:
				return nil
			}
		}

		if !isYAMLFile(path) {
			return nil
		}

		relToRepo, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		outPath := filepath.Join(outBase, relToRepo)

		rendered, err := gomplateRenderFile(opt.Gomplate, varsPath, path, opt.Verbose)
		if err != nil {
			return err
		}

		// Render/write ONLY collector workloads; skip everything else.
		if !isCollectorWorkloadYAML(rendered) {
			return nil
		}

		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(outPath, rendered, 0o644); err != nil {
			return err
		}

		if opt.WriteIndex {
			workloads = append(workloads, outPath)
		}

		return nil
	})

	return workloads, err
}

func isYAMLFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

func gomplateRenderFile(gomplateBin, varsAbsPath, inFile string, verbose bool) ([]byte, error) {
	// gomplate v5: --context expects alias=URL form; '.' sets root context.
	// For an absolute Unix path, "file://" + "/Users/..." => "file:///Users/..."
	ctxURL := "file://" + filepath.ToSlash(varsAbsPath)

	cmd := exec.Command(
		gomplateBin,
		"-c", ".="+ctxURL,
		"-f", inFile,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if verbose {
		fmt.Fprintf(os.Stderr, "gomplate cmd: %q\n", cmd.Args)
	}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf(
			"gomplate render failed for %s: %w: %s",
			inFile,
			err,
			strings.TrimSpace(stderr.String()),
		)
	}
	return stdout.Bytes(), nil
}

func isCollectorWorkloadYAML(b []byte) bool {
	dec := k8syaml.NewYAMLOrJSONDecoder(bytes.NewReader(b), 4096)

	for {
		var u unstructured.Unstructured
		if err := dec.Decode(&u); err != nil {
			if errors.Is(err, io.EOF) {
				return false
			}
			// Invalid YAML: treat as non-workload (safer than accidentally including it).
			return false
		}

		// Skip empty YAML docs
		if len(u.Object) == 0 {
			continue
		}

		if !isWorkloadKind(u.GetKind()) {
			continue
		}

		// Check object labels
		if u.GetLabels()[collectorLabelKey] == collectorLabelValue {
			return true
		}

		// Check pod template labels (common case for workloads)
		lbls, found, _ := unstructured.NestedStringMap(u.Object, "spec", "template", "metadata", "labels")
		if found && lbls[collectorLabelKey] == collectorLabelValue {
			return true
		}
	}
}

func isWorkloadKind(kind string) bool {
	_, ok := workloadKinds[kind]
	return ok
}

func mustAbs(p string) string {
	a, err := filepath.Abs(p)
	if err != nil {
		fatalf("error: cannot resolve path %q: %v\n", p, err)
	}
	return a
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

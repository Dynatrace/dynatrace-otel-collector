// internal/renderworkloads/main.go
//
// Renders YAML templates (using Go's built-in text/template) under a given input root,
// writing ONLY rendered collector workload YAMLs (Deployment/DaemonSet/StatefulSet) to an
// output directory while preserving relative paths.
// Also writes workloads.txt containing paths to rendered workload YAMLs.
//
// Values are provided via a JSON file (default: render-vars.json) located in -repo-root.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

const (
	defaultOutBase  = "/tmp/rendered"
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

	vars, err := loadVarsJSON(varsPath)
	if err != nil {
		fatalf("error: reading vars file %s: %v\n", varsPath, err)
	}

	workloads, err := renderCollectorWorkloads(repoRoot, inRoot, outBase, vars, opt)
	if err != nil {
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
	flag.StringVar(&opt.VarsFile, "vars-file", defaultVarsFile, "Vars JSON file name (resolved relative to -repo-root)")
	flag.BoolVar(&opt.WriteIndex, "write-index", true, "Write workloads.txt with rendered workload YAML paths")
	flag.BoolVar(&opt.Verbose, "verbose", false, "Verbose output (print files being rendered)")
	flag.Parse()
	return opt
}

func renderCollectorWorkloads(repoRoot, inRoot, outBase string, vars map[string]any, opt Options) ([]string, error) {
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

		if opt.Verbose {
			fmt.Fprintf(os.Stderr, "render: %s\n", path)
		}

		rendered, err := goTemplateRenderFile(path, vars)
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

func loadVarsJSON(varsAbsPath string) (map[string]any, error) {
	b, err := os.ReadFile(varsAbsPath)
	if err != nil {
		return nil, err
	}
	var v map[string]any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func goTemplateRenderFile(inFile string, vars map[string]any) ([]byte, error) {
	src, err := os.ReadFile(inFile)
	if err != nil {
		return nil, err
	}

	tpl, err := template.New(filepath.Base(inFile)).
		Option("missingkey=error").
		Funcs(templateFuncs()).
		Parse(string(src))
	if err != nil {
		return nil, fmt.Errorf("template parse failed for %s: %w", inFile, err)
	}

	var out bytes.Buffer
	if err := tpl.Execute(&out, vars); err != nil {
		return nil, fmt.Errorf("template execute failed for %s: %w", inFile, err)
	}
	return out.Bytes(), nil
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		// Strings/formatting
		"indent":  indent,
		"nindent": nindent,

		// Defaults (minimal)
		"default": defaultValue,

		// Serialization helpers
		"toYaml": toYAML,
		"toJson": toJSON,
	}
}

func indent(spaces int, s string) string {
	if spaces <= 0 || s == "" {
		return s
	}
	pad := strings.Repeat(" ", spaces)
	lines := strings.Split(s, "\n")
	for i := range lines {
		// keep trailing empty line empty (common after yaml.Marshal)
		if lines[i] == "" && i == len(lines)-1 {
			continue
		}
		lines[i] = pad + lines[i]
	}
	return strings.Join(lines, "\n")
}

func nindent(spaces int, s string) string {
	if s == "" {
		return ""
	}
	return "\n" + indent(spaces, s)
}

// defaultValue is intentionally minimal (not "deep empty") to avoid overdoing semantics.
// It only treats nil and "" as empty.
func defaultValue(def, v any) any {
	if v == nil {
		return def
	}
	if s, ok := v.(string); ok && s == "" {
		return def
	}
	return v
}

func toYAML(v any) (string, error) {
	b, err := yaml.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func toJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
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

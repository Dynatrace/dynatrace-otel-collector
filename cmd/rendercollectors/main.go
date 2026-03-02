package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	reName            = regexp.MustCompile(`\{\{\s*\.Name\s*\}\}`)
	reNamespace       = regexp.MustCompile(`\{\{\s*\.Namespace\s*\}\}`)
	reTestID          = regexp.MustCompile(`\{\{\s*\.TestID\s*\}\}`)
	reHostEndpoint    = regexp.MustCompile(`\{\{\s*\.HostEndpoint\s*\}\}`)
	reContainerReg    = regexp.MustCompile(`\{\{\s*\.ContainerRegistry\s*\}\}`)
	reK8sCluster      = regexp.MustCompile(`\{\{\s*\.K8sCluster\s*\}\}`)
	reCollectorConfig = regexp.MustCompile(`\{\{\s*\.CollectorConfig\s*\}\}`)
)

func main() {
	var (
		integrationRoot = flag.String("integration-root", "", "Path under repo root, e.g. internal/testbed/integration")
		outBase         = flag.String("out-base", "", "Output directory, e.g. /tmp/rendered-collectors")
		workloadsFile   = flag.String("workloads-file", "", "Optional override for workloads list output (default: <out-base>/workloads.txt)")
		repoRootFlag    = flag.String("repo-root", "", "Repo root directory (used to write relative paths in workloads.txt)")
	)
	flag.Parse()

	if *integrationRoot == "" || *outBase == "" {
		fatalf("missing required flags: -integration-root and -out-base")
	}

	repoRoot := *repoRootFlag
	if repoRoot == "" {
		var err error
		repoRoot, err = os.Getwd()
		if err != nil {
			fatalf("getwd: %v", err)
		}
	}
	repoRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		fatalf("abs repo-root: %v", err)
	}

	// Ensure integrationRoot is absolute for walking/copying
	integrationAbs := filepath.Clean(filepath.Join(repoRoot, *integrationRoot))
	outBaseAbs := filepath.Clean(*outBase)

	if err := os.RemoveAll(outBaseAbs); err != nil {
		fatalf("remove out-base: %v", err)
	}
	if err := os.MkdirAll(outBaseAbs, 0o755); err != nil {
		fatalf("mkdir out-base: %v", err)
	}

	collectorDirs, err := findCollectorDirs(integrationAbs)
	if err != nil {
		fatalf("find collector dirs: %v", err)
	}
	if len(collectorDirs) == 0 {
		fatalf("no collector* directories found under: %s", integrationAbs)
	}

	fmt.Printf("Found collector template dirs: %d\n", len(collectorDirs))
	for _, d := range collectorDirs {
		rel, _ := filepath.Rel(integrationAbs, d)
		fmt.Printf(" - %s\n", filepath.ToSlash(rel))
	}

	// Render all
	for _, inDir := range collectorDirs {
		rel, _ := filepath.Rel(integrationAbs, inDir)
		safe := strings.ReplaceAll(filepath.ToSlash(rel), "/", "_")
		outDir := filepath.Join(outBaseAbs, safe)

		if err := copyDir(inDir, outDir); err != nil {
			fatalf("copy %s -> %s: %v", inDir, outDir, err)
		}
		if err := preprocessYAMLs(outDir); err != nil {
			fatalf("preprocess %s: %v", outDir, err)
		}
		if err := ensureNoTemplatesRemain(outDir); err != nil {
			fatalf("template leftovers in %s: %v", inDir, err)
		}
	}

	// Collect workload YAMLs and write relative paths (relative to repo root)
	workloadAbs, err := collectWorkloadYAMLs(outBaseAbs)
	if err != nil {
		fatalf("collect workload yamls: %v", err)
	}
	if len(workloadAbs) == 0 {
		fatalf("no workload YAMLs found after preprocessing")
	}

	outList := *workloadsFile
	if outList == "" {
		outList = filepath.Join(outBaseAbs, "workloads.txt")
	}

	relList := make([]string, 0, len(workloadAbs))
	for _, p := range workloadAbs {
		r, err := filepath.Rel(repoRoot, p)
		if err != nil {
			fatalf("make relative path for %s: %v", p, err)
		}
		relList = append(relList, filepath.ToSlash(r))
	}
	sort.Strings(relList)

	if err := writeLines(outList, relList); err != nil {
		fatalf("write workloads file: %v", err)
	}

	fmt.Printf("Wrote %d workload YAMLs to %s\n", len(relList), outList)
	// Optional: print a small preview to help CI logs
	preview := 20
	if len(relList) < preview {
		preview = len(relList)
	}
	for i := 0; i < preview; i++ {
		fmt.Printf(" - %s\n", relList[i])
	}
	if len(relList) > preview {
		fmt.Printf(" ... (%d more)\n", len(relList)-preview)
	}
}

func findCollectorDirs(integrationAbs string) ([]string, error) {
	var dirs []string

	err := filepath.WalkDir(integrationAbs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}

		base := filepath.Base(path)
		if !strings.HasPrefix(base, "collector") {
			return nil
		}

		// ".../testdata/collector*"
		parent := filepath.Base(filepath.Dir(path))
		if parent != "testdata" {
			return nil
		}

		dirs = append(dirs, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(dirs)
	return dirs, nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(src, path)
		outPath := filepath.Join(dst, rel)

		info, err := d.Info()
		if err != nil {
			return err
		}

		if d.IsDir() {
			return os.MkdirAll(outPath, info.Mode().Perm())
		}

		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		return copyFile(path, outPath, info.Mode().Perm())
	})
}

func copyFile(src, dst string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}

	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func preprocessYAMLs(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		orig, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		s := string(orig)

		s = reName.ReplaceAllString(s, "otelcol-ci")
		s = reNamespace.ReplaceAllString(s, "e2e")
		s = reTestID.ReplaceAllString(s, "ci")
		s = reHostEndpoint.ReplaceAllString(s, "http://example.invalid")
		s = reContainerReg.ReplaceAllString(s, "dynatrace")
		s = reK8sCluster.ReplaceAllString(s, "ci")
		s = reCollectorConfig.ReplaceAllString(s, "receivers: {}\nexporters: {}\nservice: { pipelines: {} }\n")

		if s == string(orig) {
			return nil
		}

		// Preserve existing permissions if possible; fallback to 0644
		mode := fs.FileMode(0o644)
		if st, err := os.Stat(path); err == nil {
			mode = st.Mode().Perm()
		}
		return os.WriteFile(path, []byte(s), mode)
	})
}

func ensureNoTemplatesRemain(root string) error {
	var first []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Contains(b, []byte("{{")) {
			return nil
		}

		sc := bufio.NewScanner(bytes.NewReader(b))
		line := 0
		for sc.Scan() {
			line++
			txt := sc.Text()
			if strings.Contains(txt, "{{") {
				first = append(first, fmt.Sprintf("%s:%d:%s", filepath.ToSlash(path), line, txt))
				if len(first) >= 50 {
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(first) > 0 {
		return fmt.Errorf("unhandled template expressions remain; first occurrences:\n%s", strings.Join(first, "\n"))
	}
	return nil
}

func collectWorkloadYAMLs(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		kinds, err := kindsInFile(path)
		if err != nil {
			return fmt.Errorf("parse yaml %s: %w", path, err)
		}
		for _, k := range kinds {
			switch k {
			case "Deployment", "DaemonSet", "StatefulSet":
				out = append(out, path)
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func kindsInFile(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	dec := yaml.NewDecoder(bytes.NewReader(b))
	var kinds []string

	for {
		var doc map[string]any
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if doc == nil {
			continue
		}
		if k, ok := doc["kind"].(string); ok && k != "" {
			kinds = append(kinds, k)
		}
	}

	return kinds, nil
}

func writeLines(path string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	w := bufio.NewWriter(f)
	for _, s := range lines {
		if _, err := w.WriteString(s + "\n"); err != nil {
			return err
		}
	}
	if err := w.Flush(); err != nil {
		return err
	}
	return f.Close()
}

func fatalf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", a...)
	os.Exit(1)
}

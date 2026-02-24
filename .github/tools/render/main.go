package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

func main() {
	inDir := flag.String("in", "", "input dir containing Go-template YAMLs")
	outDir := flag.String("out", "", "output dir for rendered YAMLs (mirrors structure)")
	dataJSON := flag.String("data", "{}", "JSON object used as template data")
	flag.Parse()

	if *inDir == "" || *outDir == "" {
		fmt.Fprintln(os.Stderr, "usage: render -in <dir> -out <dir> -data <json>")
		os.Exit(2)
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(*dataJSON), &data); err != nil {
		fmt.Fprintf(os.Stderr, "invalid -data JSON: %v\n", err)
		os.Exit(2)
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir out: %v\n", err)
		os.Exit(1)
	}

	err := filepath.WalkDir(*inDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		low := strings.ToLower(d.Name())
		if !(strings.HasSuffix(low, ".yaml") || strings.HasSuffix(low, ".yml")) {
			return nil
		}

		rel, err := filepath.Rel(*inDir, path)
		if err != nil {
			return err
		}

		outPath := filepath.Join(*outDir, rel)
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}

		tmpl := template.Must(template.New(filepath.Base(path)).ParseFiles(path))
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return fmt.Errorf("template execute %s: %w", path, err)
		}

		return os.WriteFile(outPath, buf.Bytes(), 0o644)
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

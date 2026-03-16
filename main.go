package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func printHelp() {
	fmt.Print(`Usage:
	brunoc -i <collection with .bru files> -o <new collection with .yaml files>
`)
}

func collectionName(inputDir string) string {
	name := filepath.Base(inputDir)
	if name == "." || name == "/" {
		if wd, err := os.Getwd(); err == nil {
			name = filepath.Base(wd)
		}
	}

	data, err := os.ReadFile(filepath.Join(inputDir, "bruno.json"))
	if err != nil {
		return name
	}

	var manifest BrunoJSON
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.Name == "" {
		return name
	}
	return manifest.Name
}

func writeCollectionManifest(inputDir, outputDir string) error {
	rootConfig := OCCollection{
		Opencollection: "1.0.0",
		Info:           OCInfo{Name: collectionName(inputDir)},
		Config: &OCConfig{
			Proxy: OCConfigProxy{
				Inherit: true,
				Config:  OCConfigProxyConfig{Protocol: "http"},
			},
		},
		Bundled:    false,
		Extensions: make(map[string]interface{}),
	}

	data, err := marshalYAML(rootConfig)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputDir, "opencollection.yml"), data, 0644)
}

func convertFile(inputFile, outputFile string) error {
	content, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("reading %s: %w", inputFile, err)
	}

	blocks := parseBru(string(content))
	data := convertBruToData(blocks)
	if data.Name == "" {
		data.Name = strings.TrimSuffix(filepath.Base(inputFile), ".bru")
	}

	yamlOutput, err := generateYAML(data)
	if err != nil {
		return fmt.Errorf("generating YAML for %s: %w", inputFile, err)
	}

	if err := os.MkdirAll(filepath.Dir(outputFile), 0755); err != nil {
		return fmt.Errorf("creating output directory for %s: %w", outputFile, err)
	}
	if err := os.WriteFile(outputFile, []byte(yamlOutput), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", outputFile, err)
	}
	return nil
}

func main() {
	inputDir := flag.String("i", "", "Input directory or file")
	outputDir := flag.String("o", "", "Output directory")
	help := flag.Bool("h", false, "Show help")
	flag.Parse()

	if *help {
		printHelp()
		return
	}

	if *inputDir == "" || *outputDir == "" {
		printHelp()
		fmt.Fprintln(os.Stderr, "error: please provide input and output directories")
		os.Exit(1)
	}

	info, err := os.Stat(*inputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if !info.IsDir() {
		outPath := filepath.Join(*outputDir, strings.TrimSuffix(filepath.Base(*inputDir), ".bru")+".yml")
		if err := convertFile(*inputDir, outPath); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error creating output directory: %v\n", err)
		os.Exit(1)
	}

	if err := writeCollectionManifest(*inputDir, *outputDir); err != nil {
		fmt.Fprintf(os.Stderr, "error writing collection manifest: %v\n", err)
		os.Exit(1)
	}

	var conversionErrors []string
	err = filepath.WalkDir(*inputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".bru") || d.Name() == "folder.bru" {
			return nil
		}

		relPath, err := filepath.Rel(*inputDir, path)
		if err != nil {
			return err
		}

		outPath := filepath.Join(*outputDir, strings.TrimSuffix(relPath, ".bru")+".yml")
		if err := convertFile(path, outPath); err != nil {
			conversionErrors = append(conversionErrors, err.Error())
		}
		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "error walking directory: %v\n", err)
		os.Exit(1)
	}

	if len(conversionErrors) > 0 {
		fmt.Fprintf(os.Stderr, "%d file(s) failed to convert:\n", len(conversionErrors))
		for _, e := range conversionErrors {
			fmt.Fprintf(os.Stderr, "  %s\n", e)
		}
		os.Exit(1)
	}
}

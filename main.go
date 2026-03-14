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

func main() {
	inputDir := flag.String("i", "", "Input directory or file")
	outputDir := flag.String("o", "", "Output directory")
	help := flag.String("h", "help", "Help")
	flag.Parse()

	if *help != "help" {
		fmt.Printf("\n%+v\n", *help)
		printHelp()
		return
	}

	if *inputDir == "" || *outputDir == "" {
		printHelp()
		fmt.Printf("ERROR:\nPlease provide input and output directories as parameters\n\n")
		return
	}

	info, err := os.Stat(*inputDir)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if !info.IsDir() {
		convertFile(*inputDir, filepath.Join(*outputDir, strings.TrimSuffix(filepath.Base(*inputDir), ".bru")+".yml"))
		return
	}

	err = os.MkdirAll(*outputDir, 0755)
	if err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	collectionName := filepath.Base(*inputDir)
	if collectionName == "." || collectionName == "/" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Printf("Getwd error: %v\n", err)
			os.Exit(1)
		}
		collectionName = filepath.Base(wd)
	}

	brunoJsonPath := filepath.Join(*inputDir, "bruno.json")
	if brunoJsonBytes, err := os.ReadFile(brunoJsonPath); err == nil {
		var manifest BrunoJSON
		if err := json.Unmarshal(brunoJsonBytes, &manifest); err == nil {
			if manifest.Name != "" {
				collectionName = manifest.Name
			}
		}
	}

	rootConfig := OCCollection{
		Opencollection: "1.0.0",
		Info: OCInfo{
			Name: collectionName,
		},
		Config: &OCConfig{
			Proxy: OCConfigProxy{
				Inherit: true,
				Config: OCConfigProxyConfig{
					Protocol:    "http",
					Hostname:    "",
					Port:        "",
					Auth:        OCConfigProxyAuth{Username: "", Password: ""},
					BypassProxy: "",
				},
			},
		},
		Bundled:    false,
		Extensions: make(map[string]interface{}),
	}
	rootData, err := MarshalYAMLWithIndent(rootConfig)
	if err == nil {
		err = os.WriteFile(filepath.Join(*outputDir, "opencollection.yml"), rootData, 0644)
		if err != nil {
			fmt.Printf("Error writing file %s: %v\n", filepath.Join(*outputDir, "opencollection.yml"), err)
			os.Exit(1)
		}
	}

	err = filepath.WalkDir(*inputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".bru") {
			if d.Name() == "folder.bru" {
				return nil
			}

			relPath, err := filepath.Rel(*inputDir, path)
			if err != nil {
				return err
			}

			outPath := filepath.Join(*outputDir, strings.TrimSuffix(relPath, ".bru")+".yml")
			err = convertFile(path, outPath)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking directory: %v\n", err)
		os.Exit(1)
	}
}

func convertFile(inputFile, outputFile string) error {
	content, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("Error reading input file %s: %v\n", inputFile, err)
	}

	blocks := parseBru(string(content))
	data := convertBruToData(blocks)
	if data.Variables != nil && data.Name == "" {
		data.Name = strings.TrimSuffix(filepath.Base(inputFile), ".bru")
	}

	yamlOutput, err := generateYAML(data)
	if err != nil {
		return fmt.Errorf("Error generating YAML for %s: %v\n", inputFile, err)
	}

	err = os.MkdirAll(filepath.Dir(outputFile), 0755)
	if err != nil {
		return fmt.Errorf("Error creating output dir for %s: %v\n", outputFile, err)
	}

	err = os.WriteFile(outputFile, []byte(yamlOutput), 0644)
	if err != nil {
		return fmt.Errorf("Error writing file %s: %v\n", outputFile, err)
	}

	fmt.Printf("Converted %s -> %s\n", inputFile, outputFile)

	return nil
}

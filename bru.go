package main

import (
	"strconv"
	"strings"
)

type BruBlock struct {
	Name    string
	Type    string
	Content string
}

type BruBody struct {
	Type    string
	Content string
}

type BruData struct {
	Name      string
	Type      string
	Seq       int
	Method    string
	URL       string
	Headers   map[string]string
	Bodies    []BruBody
	Scripts   map[string]string
	Tests     string
	Variables map[string]string
	GRPC      map[string]string
}

func parseBru(content string) []BruBlock {
	var (
		blocks         []BruBlock
		currentBlock   *BruBlock
		contentBuilder strings.Builder
	)

	lines := strings.Split(content, "\n")
	bracketCount := 0
	inTripleQuote := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" && bracketCount == 0 && !inTripleQuote {
			continue
		}

		if currentBlock == nil {
			if strings.Contains(line, "{") {
				idx := strings.Index(line, "{")
				header := strings.TrimSpace(line[:idx])
				parts := strings.Split(header, ":")
				name := parts[0]
				blockType := ""
				if len(parts) > 1 {
					blockType = parts[1]
				}

				currentBlock = &BruBlock{Name: name, Type: blockType}
				bracketCount = 1
				rest := strings.TrimSpace(line[idx+1:])
				if rest != "" {
					contentBuilder.WriteString(rest + "\n")
					bracketCount += strings.Count(rest, "{")
					bracketCount -= strings.Count(rest, "}")
				}

				if bracketCount == 0 {
					currentBlock.Content = contentBuilder.String()
					blocks = append(blocks, *currentBlock)
					currentBlock = nil
					contentBuilder.Reset()
				}
			}
		} else {
			if strings.Contains(line, "'''") || strings.Contains(line, "\"\"\"") {
				count := strings.Count(line, "'''") + strings.Count(line, "\"\"\"")
				if count%2 != 0 {
					inTripleQuote = !inTripleQuote
				}
			}

			if !inTripleQuote {
				bracketCount += strings.Count(line, "{")
				bracketCount -= strings.Count(line, "}")
			}

			if bracketCount <= 0 {
				idx := strings.LastIndex(line, "}")
				if idx >= 0 {
					contentBuilder.WriteString(line[:idx] + "\n")
				}

				currentBlock.Content = contentBuilder.String()
				blocks = append(blocks, *currentBlock)
				currentBlock = nil
				contentBuilder.Reset()
				bracketCount = 0
				inTripleQuote = false
			} else {
				contentBuilder.WriteString(line + "\n")
			}
		}
	}

	return blocks
}

func convertBruToData(blocks []BruBlock) BruData {
	data := BruData{
		Headers: make(map[string]string),
		Bodies:  make([]BruBody, 0),
		Scripts: make(map[string]string),
		GRPC:    make(map[string]string),
	}

	methods := map[string]bool{
		"get": true, "post": true, "put": true, "delete": true,
		"patch": true, "options": true, "head": true, "grpc": true,
	}

	for _, block := range blocks {
		name := strings.TrimSpace(block.Name)
		if strings.HasPrefix(name, "body:") {
			btype := strings.TrimPrefix(name, "body:")
			data.Bodies = append(data.Bodies, BruBody{
				Type:    btype,
				Content: strings.TrimSpace(block.Content),
			})
			continue
		}

		switch name {
		case "meta":
			data.Name = extractValue(block.Content, "name")
			data.Type = extractValue(block.Content, "type")
			seqStr := extractValue(block.Content, "seq")
			if seqStr != "" {
				seq, err := strconv.Atoi(seqStr)
				if err == nil {
					data.Seq = seq
				}
			}
		case "headers":
			data.Headers = parseKeyValuePairs(block.Content)
		case "vars":
			data.Variables = parseKeyValuePairs(block.Content)
		case "body":
			data.Bodies = append(data.Bodies, BruBody{
				Type:    block.Type,
				Content: strings.TrimSpace(block.Content),
			})
		case "tests":
			data.Tests = strings.TrimSpace(block.Content)
		case "script":
			data.Scripts[block.Type] = strings.TrimSpace(block.Content)
		default:
			if methods[name] {
				if name == "grpc" {
					data.Type = "grpc"
					data.GRPC["url"] = extractValue(block.Content, "url")
					data.GRPC["method"] = extractValue(block.Content, "method")
					data.GRPC["methodType"] = extractValue(block.Content, "methodType")
				} else {
					data.Method = strings.ToUpper(name)
					data.URL = extractValue(block.Content, "url")
					if data.Type == "" {
						data.Type = "http"
					}
				}
			}
		}
	}

	return data
}

func extractValue(content, key string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+":") {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, key+":"))
			return strings.Trim(val, "\"'")
		}
	}

	return ""
}

func parseKeyValuePairs(content string) map[string]string {
	m := make(map[string]string)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) == 2 {
			m[strings.TrimSpace(parts[0])] = strings.Trim(strings.TrimSpace(parts[1]), "\"'")
		}
	}

	return m
}

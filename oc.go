package main

import (
	"strings"
)

type BrunoJSON struct {
	Version string   `json:"version"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Ignore  []string `json:"ignore"`
}

type OCConfigProxyAuth struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type OCConfigProxyConfig struct {
	Protocol    string            `yaml:"protocol"`
	Hostname    string            `yaml:"hostname"`
	Port        string            `yaml:"port"`
	Auth        OCConfigProxyAuth `yaml:"auth"`
	BypassProxy string            `yaml:"bypassProxy"`
}

type OCConfigProxy struct {
	Inherit bool                `yaml:"inherit"`
	Config  OCConfigProxyConfig `yaml:"config"`
}

type OCConfig struct {
	Proxy OCConfigProxy `yaml:"proxy"`
}

type OCCollection struct {
	Opencollection string                 `yaml:"opencollection"`
	Info           OCInfo                 `yaml:"info"`
	Config         *OCConfig              `yaml:"config,omitempty"`
	Bundled        bool                   `yaml:"bundled"`
	Extensions     map[string]interface{} `yaml:"extensions"`
}

type OCInfo struct {
	Name string `yaml:"name"`
	Type string `yaml:"type,omitempty"`
	Seq  int    `yaml:"seq,omitempty"`
}

type OCHeader struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type OCParam struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
	Type  string `yaml:"type,omitempty"`
}

type OCHttp struct {
	Method  string      `yaml:"method"`
	URL     string      `yaml:"url"`
	Headers []OCHeader  `yaml:"headers,omitempty"`
	Params  []OCParam   `yaml:"params,omitempty"`
	Body    interface{} `yaml:"body,omitempty"`
}

type OCGrpc struct {
	URL           string `yaml:"url"`
	Method        string `yaml:"method"`
	MethodType    string `yaml:"methodType,omitempty"`
	ProtoFilePath string `yaml:"protoFilePath,omitempty"`
	Message       string `yaml:"message,omitempty"`
}

type OCScript struct {
	Type string `yaml:"type"`
	Code string `yaml:"code"`
}

type OCRuntime struct {
	Variables  []OCParam  `yaml:"variables,omitempty"`
	Scripts    []OCScript `yaml:"scripts,omitempty"`
	Assertions []string   `yaml:"assertions,omitempty"`
}

type OCSettings struct {
	EncodeUrl       bool `yaml:"encodeUrl"`
	Timeout         int  `yaml:"timeout,omitempty"`
	FollowRedirects bool `yaml:"followRedirects,omitempty"`
	MaxRedirects    int  `yaml:"maxRedirects,omitempty"`
}

type OCEnvironment struct {
	Name      string    `yaml:"name"`
	Variables []OCParam `yaml:"variables"`
}

type OCRequest struct {
	Info     OCInfo      `yaml:"info"`
	Http     *OCHttp     `yaml:"http,omitempty"`
	Grpc     *OCGrpc     `yaml:"grpc,omitempty"`
	Runtime  OCRuntime   `yaml:"runtime"`
	Settings *OCSettings `yaml:"settings,omitempty"`
}

func generateYAML(data BruData) (string, error) {
	if data.Variables != nil && data.Method == "" && data.URL == "" && len(data.GRPC) == 0 && data.Type == "" {
		out := OCEnvironment{
			Name:      data.Name,
			Variables: []OCParam{},
		}

		for k, v := range data.Variables {
			out.Variables = append(out.Variables, OCParam{Name: k, Value: v})
		}

		bytes, err := MarshalYAMLWithIndent(out)
		if err != nil {
			return "", err
		}

		return string(bytes), nil
	}

	out := OCRequest{
		Info: OCInfo{
			Name: data.Name,
			Type: data.Type,
			Seq:  data.Seq,
		},
		Runtime: OCRuntime{
			Variables:  []OCParam{},
			Scripts:    []OCScript{},
			Assertions: []string{},
		},
	}

	if data.Type == "" {
		if data.Method != "" || data.URL != "" {
			out.Info.Type = "http"
		} else if len(data.GRPC) > 0 {
			out.Info.Type = "grpc"
		} else {
			out.Info.Type = "http"
		}
	}

	if out.Info.Type == "grpc" {
		url := data.GRPC["url"]
		if strings.HasPrefix(url, "grpc://") {
			if strings.Contains(url, "localhost") || strings.Contains(url, "127.0.0.1") {
				url = "http://" + strings.TrimPrefix(url, "grpc://")
			} else {
				url = "https://" + strings.TrimPrefix(url, "grpc://")
			}
		}

		out.Grpc = &OCGrpc{
			URL:        url,
			Method:     data.GRPC["method"],
			MethodType: data.GRPC["methodType"],
		}

		if btypeObj, ok := data.Body["type"]; ok && btypeObj != nil && btypeObj != "" {
			if content, ok := data.Body["content"].(string); ok && content != "" {
				actualContent := ""
				lines := strings.Split(content, "\n")
				inContent := false
				var contentLines []string
				for _, line := range lines {
					if strings.Contains(line, "content:") {
						inContent = true
						idx := strings.Index(line, "content:")
						c := line[idx+8:]
						c = strings.TrimSpace(c)
						if strings.HasPrefix(c, "'''") || strings.HasPrefix(c, "\"\"\"") {
							c = c[3:]
						}
						if (strings.HasSuffix(c, "'''") || strings.HasSuffix(c, "\"\"\"")) && len(c) >= 3 {
							c = c[:len(c)-3]
							inContent = false
						}
						if c != "" {
							contentLines = append(contentLines, c)
						}
						continue
					}
					if inContent {
						if strings.Contains(line, "'''") || strings.Contains(line, "\"\"\"") {
							idx := strings.Index(line, "'''")
							if idx == -1 {
								idx = strings.Index(line, "\"\"\"")
							}
							c := line[:idx]
							contentLines = append(contentLines, c)
							inContent = false
						} else {
							contentLines = append(contentLines, line)
						}
					}
				}
				if len(contentLines) > 0 {
					actualContent = strings.Join(contentLines, "\n")
					out.Grpc.Message = cleanBlockContent(actualContent)
				} else {
					out.Grpc.Message = cleanBlockContent(content)
				}
			}
		}

	} else {
		out.Http = &OCHttp{
			Method: data.Method,
			URL:    data.URL,
		}

		if len(data.Headers) > 0 {
			for k, v := range data.Headers {
				out.Http.Headers = append(out.Http.Headers, OCHeader{Name: k, Value: v})
			}
		} else {
			out.Http.Headers = []OCHeader{}
		}

		out.Http.Params = []OCParam{}

		btypeObj, ok := data.Body["type"]
		if ok && btypeObj != nil && btypeObj != "" {
			btype := btypeObj.(string)

			content, ok := data.Body["content"].(string)
			if ok && content != "" {
				switch btype {
				case "multipart-form":
					pairs := parseKeyValuePairs(content)
					var multipart []OCParam
					for k, v := range pairs {
						multipart = append(multipart, OCParam{Name: k, Value: v, Type: "form"})
					}
					out.Http.Body = map[string]interface{}{
						"type": "multipartForm",
						"form": multipart,
					}
				case "json":
					out.Http.Body = map[string]interface{}{
						"type": "json",
						"data": cleanBlockContent(content),
					}
				case "xml":
					out.Http.Body = map[string]interface{}{
						"type": "xml",
						"data": cleanBlockContent(content),
					}
				case "text":
					out.Http.Body = map[string]interface{}{
						"type": "text",
						"data": cleanBlockContent(content),
					}
				case "form-urlencoded":
					out.Http.Body = map[string]interface{}{
						"type": "formUrlEncoded",
						"data": cleanBlockContent(content),
					}
				case "graphql":
					out.Http.Body = map[string]interface{}{
						"type": "graphql",
						"data": cleanBlockContent(content),
					}
				}
			}
		}
	}

	if data.Variables != nil {
		for k, v := range data.Variables {
			out.Runtime.Variables = append(out.Runtime.Variables, OCParam{Name: k, Value: v})
		}
	}

	if len(data.Scripts) > 0 {
		for k, v := range data.Scripts {
			scriptType := "before-request"
			if k == "post-response" || k == "res" {
				scriptType = "after-response"
			}

			out.Runtime.Scripts = append(out.Runtime.Scripts, OCScript{
				Type: scriptType,
				Code: cleanBlockContent(v),
			})
		}
	}

	if data.Tests != "" {
		out.Runtime.Assertions = append(out.Runtime.Assertions, cleanBlockContent(data.Tests))
	}

	if out.Info.Type == "http" {
		out.Settings = &OCSettings{
			EncodeUrl:       true,
			Timeout:         30000,
			FollowRedirects: true,
			MaxRedirects:    5,
		}
	}

	bytes, err := MarshalYAMLWithIndent(out)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

func cleanBlockContent(content string) string {
	lines := strings.Split(content, "\n")
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}

	end := len(lines) - 1
	for end >= start && strings.TrimSpace(lines[end]) == "" {
		end--
	}

	if start > end {
		return ""
	}

	minIndent := -1
	for i := start; i <= end; i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}

		indentCount := 0
		for _, char := range line {
			if char == ' ' || char == '\t' {
				indentCount++
			} else {
				break
			}
		}

		if minIndent == -1 || indentCount < minIndent {
			minIndent = indentCount
		}
	}

	var sb strings.Builder
	for i := start; i <= end; i++ {
		line := lines[i]
		trimmedLine := ""

		if len(line) > minIndent && minIndent != -1 {
			trimmedLine = line[minIndent:]
		} else {
			trimmedLine = strings.TrimSpace(line)
		}

		sb.WriteString(trimmedLine)
		sb.WriteString("\n")
	}

	return sb.String()
}

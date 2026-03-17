package main

import (
	"sort"
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

// isEnvironment reports whether data represents an environment/variables file
// rather than a request file.
func isEnvironment(data BruData) bool {
	return data.Variables != nil && data.Method == "" && data.URL == "" && len(data.GRPC) == 0 && data.Type == ""
}

// inferRequestType returns the request type, falling back to inference when
// not explicitly set in the source data.
func inferRequestType(data BruData) string {
	if data.Type != "" {
		return data.Type
	}
	if len(data.GRPC) > 0 {
		return "grpc"
	}
	return "http"
}

// normalizeGRPCURL converts a grpc:// URL to http:// (localhost) or https://.
func normalizeGRPCURL(url string) string {
	if !strings.HasPrefix(url, "grpc://") {
		return url
	}
	rest := strings.TrimPrefix(url, "grpc://")
	if strings.Contains(rest, "localhost") || strings.Contains(rest, "127.0.0.1") {
		return "http://" + rest
	}
	return "https://" + rest
}

// extractGRPCMessage extracts the message body from a gRPC body block,
// stripping triple-quote delimiters if present. Returns an empty string if
// no content: key is found.
func extractGRPCMessage(content string) string {
	lines := strings.Split(content, "\n")
	var result []string
	inContent := false

	for _, line := range lines {
		if !inContent {
			idx := strings.Index(line, "content:")
			if idx == -1 {
				continue
			}
			c := strings.TrimSpace(line[idx+8:])
			if strings.HasPrefix(c, "'''") || strings.HasPrefix(c, `"""`) {
				c = c[3:]
			}
			if (strings.HasSuffix(c, "'''") || strings.HasSuffix(c, `"""`)) && len(c) >= 3 {
				c = c[:len(c)-3]
				if c != "" {
					result = append(result, c)
				}
				break
			}
			inContent = true
			if c != "" {
				result = append(result, c)
			}
			continue
		}

		if strings.Contains(line, "'''") || strings.Contains(line, `"""`) {
			idx := strings.Index(line, "'''")
			if idx == -1 {
				idx = strings.Index(line, `"""`)
			}
			result = append(result, line[:idx])
			break
		}
		result = append(result, line)
	}

	return cleanBlockContent(strings.Join(result, "\n"))
}

// scriptType maps a Bruno script block type to the OC script type name.
func scriptType(bruType string) string {
	if bruType == "post-response" || bruType == "res" {
		return "after-response"
	}
	return "before-request"
}

// buildHTTPBody constructs the body map for an HTTP request.
func buildHTTPBody(btype, content string) interface{} {
	switch btype {
	case "multipart-form":
		pairs := parseKeyValuePairs(content)
		keys := sortedKeys(pairs)
		params := make([]OCParam, 0, len(pairs))
		for _, k := range keys {
			params = append(params, OCParam{Name: k, Value: pairs[k], Type: "form"})
		}
		return map[string]interface{}{"type": "multipartForm", "form": params}
	case "json":
		return map[string]interface{}{"type": "json", "data": cleanBlockContent(content)}
	case "xml":
		return map[string]interface{}{"type": "xml", "data": cleanBlockContent(content)}
	case "text":
		return map[string]interface{}{"type": "text", "data": cleanBlockContent(content)}
	case "form-urlencoded":
		return map[string]interface{}{"type": "formUrlEncoded", "data": cleanBlockContent(content)}
	case "graphql":
		return map[string]interface{}{"type": "graphql", "data": cleanBlockContent(content)}
	}
	return nil
}

// buildGRPC constructs the gRPC request object from BruData.
func buildGRPC(data BruData) *OCGrpc {
	grpc := &OCGrpc{
		URL:        normalizeGRPCURL(data.GRPC["url"]),
		Method:     data.GRPC["method"],
		MethodType: data.GRPC["methodType"],
	}
	if len(data.Bodies) > 0 {
		body := data.Bodies[0]
		if body.Type != "" && body.Content != "" {
			msg := extractGRPCMessage(body.Content)
			if msg == "" {
				msg = cleanBlockContent(body.Content)
			}
			grpc.Message = msg
		}
	}
	return grpc
}

// buildHTTP constructs the HTTP request object from BruData.
func buildHTTP(data BruData) *OCHttp {
	headers := make([]OCHeader, 0, len(data.Headers))
	for _, k := range sortedKeys(data.Headers) {
		headers = append(headers, OCHeader{Name: k, Value: data.Headers[k]})
	}

	h := &OCHttp{
		Method:  data.Method,
		URL:     data.URL,
		Headers: headers,
		Params:  []OCParam{},
	}

	if len(data.Bodies) > 0 {
		var bodies []interface{}
		for _, b := range data.Bodies {
			if b.Type != "" && b.Content != "" {
				bodies = append(bodies, buildHTTPBody(b.Type, b.Content))
			}
		}

		if len(bodies) == 1 {
			h.Body = bodies[0]
		} else if len(bodies) > 1 {
			h.Body = bodies
		}
	}

	return h
}

// buildRuntime constructs the runtime section (variables, scripts, tests).
func buildRuntime(data BruData) OCRuntime {
	rt := OCRuntime{
		Variables:  []OCParam{},
		Scripts:    []OCScript{},
		Assertions: []string{},
	}

	for _, k := range sortedKeys(data.Variables) {
		rt.Variables = append(rt.Variables, OCParam{Name: k, Value: data.Variables[k]})
	}

	for _, k := range sortedKeys(data.Scripts) {
		rt.Scripts = append(rt.Scripts, OCScript{
			Type: scriptType(k),
			Code: cleanBlockContent(data.Scripts[k]),
		})
	}

	if data.Tests != "" {
		rt.Assertions = append(rt.Assertions, cleanBlockContent(data.Tests))
	}
	return rt
}

func generateEnvironmentYAML(data BruData) (string, error) {
	out := OCEnvironment{
		Name:      data.Name,
		Variables: make([]OCParam, 0, len(data.Variables)),
	}
	for _, k := range sortedKeys(data.Variables) {
		out.Variables = append(out.Variables, OCParam{Name: k, Value: data.Variables[k]})
	}
	b, err := marshalYAML(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func generateRequestYAML(data BruData) (string, error) {
	reqType := inferRequestType(data)
	out := OCRequest{
		Info:    OCInfo{Name: data.Name, Type: reqType, Seq: data.Seq},
		Runtime: buildRuntime(data),
	}

	if reqType == "grpc" {
		out.Grpc = buildGRPC(data)
	} else {
		out.Http = buildHTTP(data)
		out.Settings = &OCSettings{
			EncodeUrl:       true,
			Timeout:         30000,
			FollowRedirects: true,
			MaxRedirects:    5,
		}
	}

	b, err := marshalYAML(out)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// generateYAML converts BruData to a YAML string, producing either an
// environment or request document depending on the data.
func generateYAML(data BruData) (string, error) {
	if isEnvironment(data) {
		return generateEnvironmentYAML(data)
	}
	return generateRequestYAML(data)
}

// sortedKeys returns the keys of a map[string]string in sorted order.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// minLeadingWhitespace returns the count of leading space/tab characters on
// the shortest non-blank line in lines, or -1 if all lines are blank.
func minLeadingWhitespace(lines []string) int {
	min := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		n := len(line) - len(strings.TrimLeft(line, " \t"))
		if min == -1 || n < min {
			min = n
		}
	}
	return min
}

// cleanBlockContent normalises indentation and removes leading/trailing blank
// lines from a block of code or text.
func cleanBlockContent(content string) string {
	lines := strings.Split(content, "\n")

	start, end := 0, len(lines)-1
	for start <= end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end >= start && strings.TrimSpace(lines[end]) == "" {
		end--
	}
	if start > end {
		return ""
	}

	trimmed := lines[start : end+1]
	indent := minLeadingWhitespace(trimmed)
	if indent < 0 {
		indent = 0
	}

	var sb strings.Builder
	for _, line := range trimmed {
		if len(line) > indent {
			sb.WriteString(line[indent:])
		} else {
			sb.WriteString(strings.TrimSpace(line))
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

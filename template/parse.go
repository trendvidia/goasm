// Package template parses goasm templates: "# TEMPLATE Name(params)"
// blocks of assembly, and the "# ALIAS" register names they use.
package template

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

type Parameter struct {
	Name string
	Type string
}

type Instruction struct {
	Mnemonic string
	Operands []string
	Label    string // If this instruction is a label
}

type Template struct {
	Name    string
	Params  []Parameter
	Returns []Parameter
	Instrs  []Instruction
}

type Parser struct {
	scanner *bufio.Scanner
	aliases map[string]string
}

func NewParser(r io.Reader) *Parser {
	return &Parser{
		scanner: bufio.NewScanner(r),
		aliases: make(map[string]string),
	}
}

// Aliases returns the "# ALIAS name = register" bindings seen by Parse.
func (p *Parser) Aliases() map[string]string { return p.aliases }

// ParseFile parses a template file and returns its templates and aliases.
func ParseFile(path string) ([]Template, map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	p := NewParser(f)
	templates, err := p.Parse()
	if err != nil {
		return nil, nil, err
	}
	return templates, p.Aliases(), nil
}

var (
	templateRegex = regexp.MustCompile(`^#\s*TEMPLATE\s+([a-zA-Z0-9_]+)\((.*?)\)(?:\s*\((.*?)\)|\s*([a-zA-Z0-9_]+))?`)
	aliasRegex    = regexp.MustCompile(`^#\s*ALIAS\s+([a-zA-Z0-9_]+)\s*=\s*([a-zA-Z0-9_]+)`)
	instrRegex    = regexp.MustCompile(`^\s*([a-zA-Z0-9_\.]+)(\s+.*)?`)
)

func (p *Parser) Parse() ([]Template, error) {
	var templates []Template
	var currentTemplate *Template

	for p.scanner.Scan() {
		line := p.scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || (strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "# TEMPLATE") && !strings.HasPrefix(trimmed, "# ALIAS")) {
			continue
		}

		if matches := templateRegex.FindStringSubmatch(trimmed); matches != nil {
			name := matches[1]
			paramsStr := matches[2]
			var params []Parameter
			if paramsStr != "" {
				parts := strings.Split(paramsStr, ",")
				for _, part := range parts {
					part = strings.TrimSpace(part)
					kv := strings.Fields(part)
					if len(kv) != 2 {
						return nil, fmt.Errorf("invalid parameter: %s", part)
					}
					params = append(params, Parameter{Name: kv[0], Type: kv[1]})
				}
			}

			var returns []Parameter
			retStr := matches[3]
			if retStr == "" && len(matches) > 4 {
				retStr = matches[4]
			}
			if retStr != "" {
				// If it's a single type without parens (e.g. "int"), handle it
				if !strings.Contains(retStr, " ") && !strings.Contains(retStr, ",") {
					returns = append(returns, Parameter{Name: "ret0", Type: retStr})
				} else {
					parts := strings.Split(retStr, ",")
					for _, part := range parts {
						part = strings.TrimSpace(part)
						kv := strings.Fields(part)
						if len(kv) == 2 {
							returns = append(returns, Parameter{Name: kv[0], Type: kv[1]})
						} else if len(kv) == 1 {
							returns = append(returns, Parameter{Name: fmt.Sprintf("ret%d", len(returns)), Type: kv[0]})
						} else {
							return nil, fmt.Errorf("invalid return parameter: %s", part)
						}
					}
				}
			}

			templates = append(templates, Template{Name: name, Params: params, Returns: returns})
			currentTemplate = &templates[len(templates)-1]
			continue
		}

		if matches := aliasRegex.FindStringSubmatch(trimmed); matches != nil {
			p.aliases[matches[1]] = matches[2]
			continue
		}

		if currentTemplate == nil {
			continue
		}

		// Handle labels
		if strings.HasSuffix(trimmed, ":") {
			label := strings.TrimSuffix(trimmed, ":")
			currentTemplate.Instrs = append(currentTemplate.Instrs, Instruction{Label: label})
			continue
		}

		// Handle instructions
		lineContent := line
		// Only strip comments if they are not ARM64 immediates (e.g., #3)
		// A comment usually starts with # preceded by a space, and not followed by a digit.
		if idx := strings.Index(line, "#"); idx != -1 {
			// Check if it's an immediate: # followed by a digit or '-'
			if idx+1 < len(line) && (line[idx+1] >= '0' && line[idx+1] <= '9' || line[idx+1] == '-') {
				// Likely an immediate, look for another # later
				// This is still a bit naive but better.
				// Let's find a # that is preceded by space and NOT followed by digit
				lastIdx := -1
				for i := 0; i < len(line); i++ {
					if line[i] == '#' {
						if i > 0 && line[i-1] == ' ' {
							if i+1 == len(line) || !(line[i+1] >= '0' && line[i+1] <= '9' || line[i+1] == '-') {
								lastIdx = i
								break
							}
						}
					}
				}
				if lastIdx != -1 {
					lineContent = line[:lastIdx]
				}
			} else {
				lineContent = line[:idx]
			}
		}
		lineContent = strings.TrimSpace(lineContent)
		if lineContent == "" {
			continue
		}

		parts := strings.Fields(lineContent)
		mnemonic := parts[0]
		var operands []string
		if len(parts) > 1 {
			opsStr := strings.Join(parts[1:], " ")
			// Split by comma, but be careful with memory operands like [BP, slot, LSL #3]
			// Simplified splitting for now, might need more robust parsing if it gets complex
			operands = parseOperands(opsStr)
		}

		currentTemplate.Instrs = append(currentTemplate.Instrs, Instruction{
			Mnemonic: mnemonic,
			Operands: operands,
		})
	}

	return templates, p.scanner.Err()
}

func parseOperands(s string) []string {
	var operands []string
	var current strings.Builder
	bracketLevel := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '[':
			bracketLevel++
			current.WriteByte(c)
		case ']':
			bracketLevel--
			current.WriteByte(c)
		case ',':
			if bracketLevel == 0 {
				operands = append(operands, strings.TrimSpace(current.String()))
				current.Reset()
			} else {
				current.WriteByte(c)
			}
		default:
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		operands = append(operands, strings.TrimSpace(current.String()))
	}
	return operands
}

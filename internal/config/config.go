package config

import (
	"fmt"
	"regexp"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version  string                   `yaml:"version"`
	Source   SourceConfig             `yaml:"source"`  // default source (single-source configs)
	Sources  map[string]SourceConfig  `yaml:"sources"` // named sources map
	Output   OutputConfig             `yaml:"output"`
	Handlers map[string]HandlerConfig `yaml:"handlers"`
	Vars     map[string]yaml.Node     `yaml:"vars"` // available to jq as $name; any YAML value
}

// varName matches names usable as jq variables.
var varName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// CheckVarName reports whether name can be used as a jq variable ($name).
// ENV is reserved for the environment.
func CheckVarName(name string) error {
	if !varName.MatchString(name) {
		return fmt.Errorf("invalid variable name %q: use letters, digits and underscores, not starting with a digit", name)
	}
	if name == "ENV" {
		return fmt.Errorf("variable name %q is reserved ($ENV is the environment)", name)
	}
	return nil
}

type SourceConfig struct {
	Path    string            `yaml:"path"`
	URL     string            `yaml:"url"`
	Format  string            `yaml:"format"`
	Method  string            `yaml:"method"`
	Headers map[string]string `yaml:"headers"`
	Body    string            `yaml:"body"`
	Timeout string            `yaml:"timeout"` // Go duration, e.g. "30s"; "0" disables; default 30s

	Delimiter string `yaml:"delimiter"` // csv only: single character; default "," ("\t" for .tsv)
	Header    *bool  `yaml:"header"`    // csv only: first row names the columns; default true

	// FormatDetected reports that Format was inferred from the path or URL
	// extension rather than set in the config.
	FormatDetected bool `yaml:"-"`
}

// DefaultTimeout is the URL source timeout used when none is configured.
const DefaultTimeout = 30 * time.Second

// TimeoutDuration returns the configured URL timeout, or DefaultTimeout when
// unset. The value has already been validated by Load.
func (s SourceConfig) TimeoutDuration() time.Duration {
	if s.Timeout == "" {
		return DefaultTimeout
	}
	d, _ := time.ParseDuration(s.Timeout)
	return d
}

type OutputConfig struct {
	Format    string `yaml:"format"`
	Indent    *int   `yaml:"indent"` // nil = default (2); 0 = compact
	File      string `yaml:"file"`
	Delimiter string `yaml:"delimiter"` // csv only: single character; default ","
}

// DelimiterRune returns the CSV delimiter, or ',' when unset.
func (o OutputConfig) DelimiterRune() rune { return delimiterRune(o.Delimiter) }

// DelimiterRune returns the CSV delimiter, or ',' when unset.
func (s SourceConfig) DelimiterRune() rune { return delimiterRune(s.Delimiter) }

// HasHeader reports whether the first CSV row names the columns.
func (s SourceConfig) HasHeader() bool { return s.Header == nil || *s.Header }

func delimiterRune(d string) rune {
	for _, r := range d {
		return r
	}
	return ','
}

// checkDelimiter validates a CSV delimiter: one character that is not a
// quote or line break.
func checkDelimiter(label, d string) error {
	if d == "" {
		return nil
	}
	if utf8.RuneCountInString(d) != 1 || d == "\"" || d == "\r" || d == "\n" || d == string(utf8.RuneError) {
		return fmt.Errorf("config: %s: 'delimiter' must be a single character other than a quote or line break (got %q)", label, d)
	}
	return nil
}

type HandlerConfig struct {
	Source string `yaml:"source"` // key into Config.Sources; empty = use Config.Source
	Input  string `yaml:"input"`
	Each   bool   `yaml:"each"`
	Filter string `yaml:"filter"`
	Fields Fields `yaml:"fields"`
}

// Fields is an ordered list of output fields, kept in config order.
type Fields []Field

// Field is a named output field.
type Field struct {
	Name string
	FieldConfig
}

// Get returns the field called name and whether it exists.
func (fs Fields) Get(name string) (FieldConfig, bool) {
	for _, f := range fs {
		if f.Name == name {
			return f.FieldConfig, true
		}
	}
	return FieldConfig{}, false
}

// UnmarshalYAML decodes a mapping of field name to field definition,
// preserving key order. Duplicate names are rejected.
func (fs *Fields) UnmarshalYAML(n *yaml.Node) error {
	n = resolveAlias(n)
	if n.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: 'fields' must be a map of field name to definition", n.Line)
	}
	out := make(Fields, 0, len(n.Content)/2)
	seen := make(map[string]bool, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		if k.Kind != yaml.ScalarNode {
			return fmt.Errorf("line %d: field names must be strings", k.Line)
		}
		if seen[k.Value] {
			return fmt.Errorf("line %d: duplicate field %q", k.Line, k.Value)
		}
		seen[k.Value] = true
		if v.Kind == yaml.ScalarNode && v.ShortTag() == "!!null" {
			return fmt.Errorf("line %d: field %q has no value; expected a jq expression", k.Line, k.Value)
		}
		var fc FieldConfig
		if err := v.Decode(&fc); err != nil {
			return err
		}
		out = append(out, Field{Name: k.Value, FieldConfig: fc})
	}
	*fs = out
	return nil
}

// FieldConfig is a union: a jq expression string, a sub-handler reference, or an inline nested object.
type FieldConfig struct {
	Expr    string
	Handler string
	Input   string
	Fields  Fields // inline nested object
}

// UnmarshalYAML decodes a field from a scalar (jq expression) or a mapping
// with 'handler' (sub-handler reference) or 'fields' (inline object) and an
// optional 'input'. Unknown keys are rejected.
func (f *FieldConfig) UnmarshalYAML(n *yaml.Node) error {
	switch n.Kind {
	case yaml.ScalarNode:
		if n.ShortTag() == "!!null" {
			return fmt.Errorf("line %d: field value is empty; expected a jq expression", n.Line)
		}
		f.Expr = n.Value
		return nil
	case yaml.MappingNode:
	case yaml.AliasNode:
		return f.UnmarshalYAML(resolveAlias(n))
	default:
		return fmt.Errorf("line %d: field must be a jq expression string, {handler, input} mapping, or {input, fields} inline object", n.Line)
	}

	var handlerNode, inputNode, fieldsNode *yaml.Node
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		switch k.Value {
		case "handler":
			handlerNode = v
		case "input":
			inputNode = v
		case "fields":
			fieldsNode = v
		default:
			return fmt.Errorf("line %d: unknown key %q in field (valid keys: handler, input, fields)", k.Line, k.Value)
		}
	}

	if inputNode != nil {
		inputNode = resolveAlias(inputNode)
		if inputNode.Kind != yaml.ScalarNode || inputNode.ShortTag() == "!!null" {
			return fmt.Errorf("line %d: field 'input' must be a jq expression string", inputNode.Line)
		}
		f.Input = inputNode.Value
	}

	switch {
	case handlerNode != nil && fieldsNode != nil:
		return fmt.Errorf("line %d: field cannot have both 'handler' and 'fields' keys", n.Line)
	case handlerNode != nil:
		handlerNode = resolveAlias(handlerNode)
		if handlerNode.Kind != yaml.ScalarNode || handlerNode.ShortTag() != "!!str" || handlerNode.Value == "" {
			return fmt.Errorf("line %d: field 'handler' must be a non-empty string", handlerNode.Line)
		}
		f.Handler = handlerNode.Value
		return nil
	case fieldsNode != nil:
		fieldsNode = resolveAlias(fieldsNode)
		if fieldsNode.Kind != yaml.MappingNode || len(fieldsNode.Content) == 0 {
			return fmt.Errorf("line %d: inline 'fields' must be a non-empty map", fieldsNode.Line)
		}
		return fieldsNode.Decode(&f.Fields)
	}
	return fmt.Errorf("line %d: field map must have a 'handler' key (sub-handler) or a 'fields' key (inline object)", n.Line)
}

func resolveAlias(n *yaml.Node) *yaml.Node {
	for n.Kind == yaml.AliasNode && n.Alias != nil {
		n = n.Alias
	}
	return n
}

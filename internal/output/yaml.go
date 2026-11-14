package output

import (
	"fmt"
	"io"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"unicode/utf8"

	"github.com/suprbdev/alpakr/internal/record"
	"gopkg.in/yaml.v3"
)

// YAMLWriter writes YAML documents. Objects produced by handlers keep their
// config field order.
type YAMLWriter struct {
	Out io.Writer
}

// Write encodes v as a single YAML document.
func (w *YAMLWriter) Write(v interface{}) error {
	n, err := yamlNode(v)
	if err != nil {
		return fmt.Errorf("encoding YAML: %w", err)
	}
	data, err := yaml.Marshal(n)
	if err != nil {
		return fmt.Errorf("encoding YAML: %w", err)
	}
	_, err = w.Out.Write(data)
	return err
}

// WriteOne writes a single record as a YAML document with a separator.
func (w *YAMLWriter) WriteOne(v interface{}) error {
	if _, err := fmt.Fprintln(w.Out, "---"); err != nil {
		return err
	}
	return w.Write(v)
}

// yamlNode builds the node tree for v. *big.Int becomes a plain scalar:
// yaml.v3 would otherwise quote it via encoding.TextMarshaler, and an
// explicit !!int tag on values beyond 64 bits is emitted verbatim and
// rejected when read back.
func yamlNode(v interface{}) (*yaml.Node, error) {
	switch x := v.(type) {
	case *big.Int:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: x.String()}, nil
	case *record.Object:
		n := &yaml.Node{Kind: yaml.MappingNode, Content: make([]*yaml.Node, 0, 2*len(x.Keys))}
		for i, k := range x.Keys {
			if err := appendPair(n, k, x.Values[i]); err != nil {
				return nil, err
			}
		}
		return n, nil
	case map[string]interface{}:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		n := &yaml.Node{Kind: yaml.MappingNode, Content: make([]*yaml.Node, 0, 2*len(keys))}
		for _, k := range keys {
			if err := appendPair(n, k, x[k]); err != nil {
				return nil, err
			}
		}
		return n, nil
	case []interface{}:
		n := &yaml.Node{Kind: yaml.SequenceNode, Content: make([]*yaml.Node, len(x))}
		for i, e := range x {
			c, err := yamlNode(e)
			if err != nil {
				return nil, err
			}
			n.Content[i] = c
		}
		return n, nil
	}
	if n := scalarNode(v); n != nil {
		return n, nil
	}
	n := &yaml.Node{}
	if err := n.Encode(v); err != nil {
		return nil, err
	}
	return n, nil
}

// scalarNode builds nodes for common scalars directly, producing the same
// output as yaml.Node.Encode without its per-call encoder setup. It returns
// nil for anything else.
func scalarNode(v interface{}) *yaml.Node {
	switch x := v.(type) {
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: "null"}
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: strconv.FormatBool(x)}
	case int:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: strconv.Itoa(x)}
	case float64:
		s := strconv.FormatFloat(x, 'g', -1, 64)
		switch s {
		case "+Inf":
			s = ".inf"
		case "-Inf":
			s = "-.inf"
		case "NaN":
			s = ".nan"
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Value: s}
	case string:
		if !utf8.ValidString(x) {
			return nil
		}
		n := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: x}
		if yaml11Ambiguous(x) {
			n.Style = yaml.DoubleQuotedStyle
		}
		return n
	}
	return nil
}

var base60Float = regexp.MustCompile(`^[-+]?[0-9][0-9_]*(?::[0-5]?[0-9])+(?:\.[0-9_]*)?$`)

// yaml11Ambiguous reports strings that YAML 1.1 readers would take as
// booleans or sexagesimal numbers; yaml.v3 quotes these when encoding
// values but not when encoding nodes.
func yaml11Ambiguous(s string) bool {
	switch s {
	case "y", "Y", "yes", "Yes", "YES", "on", "On", "ON",
		"n", "N", "no", "No", "NO", "off", "Off", "OFF":
		return true
	}
	return base60Float.MatchString(s)
}

func appendPair(n *yaml.Node, key string, v interface{}) error {
	k := scalarNode(key)
	if k == nil {
		k = &yaml.Node{}
		if err := k.Encode(key); err != nil {
			return err
		}
	}
	val, err := yamlNode(v)
	if err != nil {
		return err
	}
	n.Content = append(n.Content, k, val)
	return nil
}

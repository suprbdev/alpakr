package source

import (
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// yamlConverter turns a yaml.Node tree into gojq-compatible values. Unlike
// yaml.v3's interface{} decoding it keeps integers beyond 64 bits exact,
// keeps timestamps as their source text, stringifies non-string keys and
// rejects keys that collide once stringified.
type yamlConverter struct {
	anchors map[*yaml.Node]anchorValue
	active  map[*yaml.Node]bool
	// nodes counts the document as if every alias were expanded in place,
	// which is how jq and the output writers will traverse it.
	nodes    int
	maxNodes int
}

// anchorValue is a converted anchored node and the expanded node count it
// contributes each time it is referenced.
type anchorValue struct {
	v     interface{}
	nodes int
}

// Aliases may expand a document to at most maxExpansion times its own node
// count, or minExpandedNodes nodes if that is larger; beyond that it is
// treated as an alias bomb.
const (
	maxExpansion     = 50
	minExpandedNodes = 1_000_000
)

// YAMLNodeValue converts a YAML node (e.g. a config value) to a jq value with
// the same rules as YAML input: exact integers, timestamps as text, string
// keys.
func YAMLNodeValue(n *yaml.Node) (interface{}, error) {
	return yamlToValue(n)
}

func yamlToValue(doc *yaml.Node) (interface{}, error) {
	c := &yamlConverter{
		anchors:  map[*yaml.Node]anchorValue{},
		active:   map[*yaml.Node]bool{},
		maxNodes: max(minExpandedNodes, maxExpansion*countNodes(doc)),
	}
	return c.value(doc)
}

// countNodes counts the nodes physically present, not following aliases.
func countNodes(n *yaml.Node) int {
	total := 1
	for _, c := range n.Content {
		total += countNodes(c)
	}
	return total
}

func (c *yamlConverter) value(n *yaml.Node) (interface{}, error) {
	startNodes := c.nodes
	c.nodes++
	if n.Anchor != "" {
		if a, ok := c.anchors[n]; ok {
			c.nodes += a.nodes - 1
			return a.v, c.checkExpansion(n)
		}
		if c.active[n] {
			return nil, fmt.Errorf("line %d: anchor %q refers to itself", n.Line, n.Anchor)
		}
		c.active[n] = true
		defer delete(c.active, n)
	}

	var v interface{}
	var err error
	switch n.Kind {
	case 0:
		return nil, nil
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil, nil
		}
		v, err = c.value(n.Content[0])
	case yaml.AliasNode:
		v, err = c.value(n.Alias)
	case yaml.SequenceNode:
		v, err = c.sequence(n)
	case yaml.MappingNode:
		v, err = c.mapping(n)
	case yaml.ScalarNode:
		v, err = yamlScalar(n)
	default:
		err = fmt.Errorf("line %d: unsupported YAML node", n.Line)
	}
	if err != nil {
		return nil, err
	}
	if n.Anchor != "" {
		c.anchors[n] = anchorValue{v: v, nodes: c.nodes - startNodes}
	}
	return v, nil
}

func (c *yamlConverter) checkExpansion(n *yaml.Node) error {
	if c.nodes > c.maxNodes {
		return fmt.Errorf("line %d: document contains excessive aliasing (expands to more than %d nodes)", n.Line, c.maxNodes)
	}
	return nil
}

func (c *yamlConverter) sequence(n *yaml.Node) (interface{}, error) {
	out := make([]interface{}, len(n.Content))
	for i, item := range n.Content {
		v, err := c.value(item)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// mapping converts a mapping node. Explicit keys take precedence over keys
// merged in with "<<"; among merged mappings the first one listed wins.
func (c *yamlConverter) mapping(n *yaml.Node) (interface{}, error) {
	out := make(map[string]interface{}, len(n.Content)/2)
	var merges []*yaml.Node
	for i := 0; i+1 < len(n.Content); i += 2 {
		kn, vn := n.Content[i], n.Content[i+1]
		if kn.Kind == yaml.ScalarNode && kn.ShortTag() == "!!merge" {
			merges = append(merges, vn)
			continue
		}
		key, err := c.key(kn)
		if err != nil {
			return nil, err
		}
		if _, dup := out[key]; dup {
			return nil, fmt.Errorf("line %d: duplicate mapping key %q", kn.Line, key)
		}
		v, err := c.value(vn)
		if err != nil {
			return nil, err
		}
		out[key] = v
	}

	for _, mn := range merges {
		sources := []*yaml.Node{mn}
		if mn.Kind == yaml.SequenceNode {
			sources = mn.Content
		}
		for _, sn := range sources {
			v, err := c.value(sn)
			if err != nil {
				return nil, err
			}
			m, ok := v.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("line %d: merge key '<<' value must be a mapping", sn.Line)
			}
			for k, mv := range m {
				if _, exists := out[k]; !exists {
					out[k] = mv
				}
			}
		}
	}
	return out, nil
}

func (c *yamlConverter) key(n *yaml.Node) (string, error) {
	if n.Kind == yaml.AliasNode {
		n = n.Alias
	}
	if n.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("line %d: mapping keys must be scalars", n.Line)
	}
	v, err := yamlScalar(n)
	if err != nil {
		return "", err
	}
	switch k := v.(type) {
	case nil:
		return "null", nil
	case string:
		return k, nil
	case float64:
		return strconv.FormatFloat(k, 'g', -1, 64), nil
	}
	return fmt.Sprint(v), nil
}

// plainInt matches decimal integers that yaml.v3 resolves as !!float because
// they overflow 64 bits.
var plainInt = regexp.MustCompile(`^[-+]?[0-9][0-9_]*$`)

func yamlScalar(n *yaml.Node) (interface{}, error) {
	switch n.ShortTag() {
	case "!!str", "!!timestamp":
		return n.Value, nil
	case "!!null":
		return nil, nil
	case "!!bool":
		switch n.Value {
		case "true", "True", "TRUE":
			return true, nil
		case "false", "False", "FALSE":
			return false, nil
		}
	case "!!int":
		s := strings.ReplaceAll(n.Value, "_", "")
		if i, err := strconv.ParseInt(s, 0, 0); err == nil {
			return int(i), nil
		}
		if b, ok := new(big.Int).SetString(s, 0); ok {
			return b, nil
		}
	case "!!float":
		if n.Style&(yaml.TaggedStyle|yaml.SingleQuotedStyle|yaml.DoubleQuotedStyle) == 0 && plainInt.MatchString(n.Value) {
			if b, ok := new(big.Int).SetString(strings.ReplaceAll(n.Value, "_", ""), 10); ok {
				return b, nil
			}
		}
		if f, err := strconv.ParseFloat(strings.ReplaceAll(n.Value, "_", ""), 64); err == nil && !math.IsInf(f, 0) {
			return f, nil
		}
	}
	var v interface{}
	if err := n.Decode(&v); err != nil {
		return nil, err
	}
	switch x := v.(type) {
	case int64:
		return new(big.Int).SetInt64(x), nil
	case uint64:
		return new(big.Int).SetUint64(x), nil
	case time.Time:
		return n.Value, nil
	case []byte:
		return string(x), nil
	}
	return v, nil
}

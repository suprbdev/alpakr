package config

import (
	"fmt"
	"os"
	"regexp"
	"sort"

	"gopkg.in/yaml.v3"
)

var envRef = regexp.MustCompile(`\$\$|\$\{([A-Za-z_][A-Za-z0-9_]*)(:-([^}]*))?\}`)

// expandEnv replaces ${VAR} with the value of the environment variable VAR
// and ${VAR:-default} with default when VAR is unset or empty. $$ yields a
// literal $. Other uses of $ are left untouched. label names the setting in
// errors.
func expandEnv(label, s string) (string, error) {
	var missing string
	out := envRef.ReplaceAllStringFunc(s, func(ref string) string {
		if ref == "$$" {
			return "$"
		}
		m := envRef.FindStringSubmatch(ref)
		name, hasDefault, def := m[1], m[2] != "", m[3]
		v, ok := os.LookupEnv(name)
		if hasDefault && v == "" {
			return def
		}
		if !ok && missing == "" {
			missing = name
		}
		return v
	})
	if missing != "" {
		return "", fmt.Errorf("config: %s: environment variable %q is not set (use ${%s:-default} for a fallback)", label, missing, missing)
	}
	return out, nil
}

func expandSourceEnv(label string, s *SourceConfig) error {
	for _, f := range []struct {
		name string
		v    *string
	}{
		{"path", &s.Path},
		{"url", &s.URL},
		{"method", &s.Method},
		{"body", &s.Body},
		{"timeout", &s.Timeout},
	} {
		v, err := expandEnv(label+"."+f.name, *f.v)
		if err != nil {
			return err
		}
		*f.v = v
	}
	keys := make([]string, 0, len(s.Headers))
	for k := range s.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v, err := expandEnv(label+".headers."+k, s.Headers[k])
		if err != nil {
			return err
		}
		s.Headers[k] = v
	}
	return nil
}

// expandNodeEnv expands environment references in every string scalar of n.
func expandNodeEnv(label string, n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode && n.ShortTag() == "!!str" {
		v, err := expandEnv(label, n.Value)
		if err != nil {
			return err
		}
		n.Value = v
	}
	for _, c := range n.Content {
		if err := expandNodeEnv(label, c); err != nil {
			return err
		}
	}
	return nil
}

func sortedNames(m map[string]yaml.Node) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// expandConfigEnv expands environment references in source settings, vars
// and output.file.
func expandConfigEnv(cfg *Config) error {
	if err := expandSourceEnv("source", &cfg.Source); err != nil {
		return err
	}
	names := make([]string, 0, len(cfg.Sources))
	for name := range cfg.Sources {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		s := cfg.Sources[name]
		if err := expandSourceEnv("sources."+name, &s); err != nil {
			return err
		}
		cfg.Sources[name] = s
	}
	for _, name := range sortedNames(cfg.Vars) {
		n := cfg.Vars[name]
		if err := expandNodeEnv("vars."+name, &n); err != nil {
			return err
		}
		cfg.Vars[name] = n
	}
	file, err := expandEnv("output.file", cfg.Output.File)
	if err != nil {
		return err
	}
	cfg.Output.File = file
	return nil
}

package config

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var unknownFieldRe = regexp.MustCompile(`^line (\d+): field (\S+) not found in type config\.(\w+)$`)

// sections maps config struct names to the section they decode, for error
// messages.
var sections = map[string]reflect.Type{
	"top level": reflect.TypeOf(Config{}),
	"source":    reflect.TypeOf(SourceConfig{}),
	"output":    reflect.TypeOf(OutputConfig{}),
	"handler":   reflect.TypeOf(HandlerConfig{}),
}

// friendlyYAMLError rewrites yaml.v3's unknown-field errors, which name Go
// types, into the config section and its valid keys.
func friendlyYAMLError(err error) error {
	var te *yaml.TypeError
	if !errors.As(err, &te) {
		return err
	}
	msgs := make([]string, len(te.Errors))
	for i, e := range te.Errors {
		msgs[i] = e
		m := unknownFieldRe.FindStringSubmatch(e)
		if m == nil {
			continue
		}
		for section, t := range sections {
			if t.Name() == m[3] {
				msgs[i] = fmt.Sprintf("line %s: unknown key %q in %s (valid keys: %s)", m[1], m[2], section, strings.Join(yamlKeys(t), ", "))
			}
		}
	}
	return errors.New(strings.Join(msgs, "; "))
}

func yamlKeys(t reflect.Type) []string {
	keys := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		if tag, _, _ := strings.Cut(t.Field(i).Tag.Get("yaml"), ","); tag != "" && tag != "-" {
			keys = append(keys, tag)
		}
	}
	return keys
}

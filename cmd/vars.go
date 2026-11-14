package cmd

import (
	"fmt"
	"strings"

	"github.com/suprbdev/alpakr/internal/config"
	"github.com/suprbdev/alpakr/internal/source"
)

var (
	argStrings []string
	argJSON    []string
)

func init() {
	rootCmd.PersistentFlags().StringArrayVar(&argStrings, "arg", nil, "set jq variable $name to a string: --arg name=value (repeatable)")
	rootCmd.PersistentFlags().StringArrayVar(&argJSON, "argjson", nil, "set jq variable $name to a JSON value: --argjson name='{\"a\":1}' (repeatable)")
}

// buildVars merges the config's vars with --arg and --argjson, which take
// precedence.
func buildVars(cfg *config.Config) (map[string]interface{}, error) {
	vars := make(map[string]interface{}, len(cfg.Vars)+len(argStrings)+len(argJSON))
	for name, node := range cfg.Vars {
		v, err := source.YAMLNodeValue(&node)
		if err != nil {
			return nil, fmt.Errorf("config: vars.%s: %w", name, err)
		}
		vars[name] = v
	}
	for _, a := range argStrings {
		name, value, err := splitArg("--arg", a)
		if err != nil {
			return nil, err
		}
		vars[name] = value
	}
	for _, a := range argJSON {
		name, value, err := splitArg("--argjson", a)
		if err != nil {
			return nil, err
		}
		v, err := source.ParseJSON([]byte(value))
		if err != nil {
			return nil, fmt.Errorf("--argjson %s: %w", name, err)
		}
		vars[name] = v
	}
	return vars, nil
}

func splitArg(flag, a string) (string, string, error) {
	name, value, ok := strings.Cut(a, "=")
	if !ok {
		return "", "", fmt.Errorf("%s %q: want name=value", flag, a)
	}
	if err := config.CheckVarName(name); err != nil {
		return "", "", fmt.Errorf("%s: %w", flag, err)
	}
	return name, value, nil
}

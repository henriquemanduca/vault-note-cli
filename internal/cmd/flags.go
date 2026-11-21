package cmd

import (
	"fmt"
	"strconv"
	"strings"
)

type globalOpts struct {
	format     OutputFormat
	configPath string
	vaultPath  string
}

// extractGlobalFlags strips transversal flags (-F/--format, --vault, and --config)
// from anywhere in args before `--`, returning the remaining arguments.
func extractGlobalFlags(args []string) (globalOpts, []string, error) {
	opts := globalOpts{format: FormatTSV}
	rest := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			rest = append(rest, args[i:]...)
			break
		}

		switch {
		case arg == "-F" || arg == "--format":
			if i+1 >= len(args) {
				return opts, nil, UsageError("flag %s requires a value (tsv or table)", arg)
			}
			i++
			f, err := parseOutputFormat(args[i])
			if err != nil {
				return opts, nil, err
			}
			opts.format = f
		case strings.HasPrefix(arg, "--format="):
			f, err := parseOutputFormat(strings.TrimPrefix(arg, "--format="))
			if err != nil {
				return opts, nil, err
			}
			opts.format = f
		case strings.HasPrefix(arg, "-F="):
			f, err := parseOutputFormat(strings.TrimPrefix(arg, "-F="))
			if err != nil {
				return opts, nil, err
			}
			opts.format = f
		case arg == "--config":
			if i+1 >= len(args) {
				return opts, nil, UsageError("flag --config requires a YAML file path")
			}
			i++
			opts.configPath = args[i]
		case strings.HasPrefix(arg, "--config="):
			opts.configPath = strings.TrimPrefix(arg, "--config=")
		case arg == "--vault":
			if i+1 >= len(args) {
				return opts, nil, UsageError("flag --vault requires a directory path")
			}
			i++
			opts.vaultPath = args[i]
		case strings.HasPrefix(arg, "--vault="):
			opts.vaultPath = strings.TrimPrefix(arg, "--vault=")
		default:
			rest = append(rest, arg)
		}
	}
	return opts, rest, nil
}

func parseOutputFormat(raw string) (OutputFormat, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "tsv":
		return FormatTSV, nil
	case "table":
		return FormatTable, nil
	default:
		return "", UsageError("invalid format %q (expected tsv or table)", raw)
	}
}

// splitFlagsAndPosArgs separates flags from positional arguments so flags can
// appear before, between, or after positional arguments (respecting `--`).
// valueFlags contains flag names (without leading dashes) that consume a value.
func splitFlagsAndPosArgs(args []string, valueFlags map[string]bool) (flagArgs, posArgs []string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			posArgs = append(posArgs, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			flagArgs = append(flagArgs, arg)
			if strings.Contains(arg, "=") {
				continue
			}
			name := strings.TrimLeft(arg, "-")
			if valueFlags[name] && i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
			continue
		}
		posArgs = append(posArgs, arg)
	}
	return flagArgs, posArgs
}

// optionalStringFlag distinguishes between omitted flag vs explicitly set to "".
type optionalStringFlag struct {
	set   bool
	value string
}

func (o *optionalStringFlag) String() string {
	return o.value
}

func (o *optionalStringFlag) Set(val string) error {
	o.set = true
	o.value = val
	return nil
}

// optionalIntFlag distinguishes between omitted flag vs explicitly set to 0.
type optionalIntFlag struct {
	set   bool
	value int
}

func (o *optionalIntFlag) String() string {
	return strconv.Itoa(o.value)
}

func (o *optionalIntFlag) Set(val string) error {
	n, err := strconv.Atoi(val)
	if err != nil {
		return fmt.Errorf("expected an integer, got %q", val)
	}
	o.set = true
	o.value = n
	return nil
}

func hasHelpFlag(args []string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if a == "-h" || a == "--help" {
			return true
		}
	}
	return false
}

func resolveNotebookDefault(ctx *Context, flagVal optionalStringFlag) string {
	if flagVal.set {
		return strings.TrimSpace(flagVal.value)
	}
	if ctx.Config != nil {
		return strings.TrimSpace(ctx.Config.Notebook)
	}
	return ""
}

package cmd

// FlatAliases expands frequent shorthand commands into their canonical
// hierarchical routes (<resource> <action>) before dispatch.
var FlatAliases = map[string][]string{
	"add":    {"note", "add"},
	"new":    {"note", "add"},
	"n":      {"note", "add"},
	"list":   {"note", "list"},
	"ls":     {"note", "list"},
	"view":   {"note", "view"},
	"show":   {"note", "view"},
	"get":    {"note", "view"},
	"update": {"note", "update"},
	"mod":    {"note", "update"},
	"mv":     {"note", "update"},
	"edit":   {"note", "edit"},
	"delete": {"note", "delete"},
	"del":    {"note", "delete"},
	"rm":     {"note", "delete"},
	"search": {"note", "search"},
	"find":   {"note", "search"},
	"nb":     {"notebook"},
	"nbs":    {"notebook", "list"},
	"ln":     {"notebook", "list"},
	"nn":     {"notebook", "add"},
	"cfg":    {"config"},
	"ui":     {"tui"},
}

func expandAliases(args []string) []string {
	if len(args) == 0 {
		return args
	}
	if expanded, ok := FlatAliases[args[0]]; ok {
		out := make([]string, 0, len(expanded)+len(args)-1)
		out = append(out, expanded...)
		out = append(out, args[1:]...)
		return out
	}
	return args
}

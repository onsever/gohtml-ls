package goanalysis

// BuiltinFuncs returns documentation for Go template built-in functions.
func BuiltinFuncs() map[string]FuncSig {
	return map[string]FuncSig{
		"and": {Name: "and", Signature: "and x y ...", Doc: "Returns the boolean AND of its arguments. Returns the first empty argument or the last argument.\n\n**Example:**\n```\n{{if and .IsAdmin .IsActive}}...{{end}}\n```"},
		"call": {Name: "call", Signature: "call fn arg...", Doc: "Calls the first argument (which must be a function) with the remaining arguments as parameters. The function must return 1 or 2 values (the second being an error).\n\n**Example:**\n```\n{{call .Formatter .Value}}\n```"},
		"html": {Name: "html", Signature: "html args...", Doc: "Returns the escaped HTML equivalent of the textual representation of its arguments.\n\n**Example:**\n```\n{{html .UserInput}}\n{{\"<b>bold</b>\" | html}}  → &lt;b&gt;bold&lt;/b&gt;\n```"},
		"index": {Name: "index", Signature: "index x i...", Doc: "Returns the result of indexing the first argument by the following indices. Each index must be appropriate for the type (int for slices/arrays, key type for maps).\n\n**Example:**\n```\n{{index .Items 0}}          → first element\n{{index .Matrix 1 2}}       → .Matrix[1][2]\n{{index .Map \"key\"}}         → .Map[\"key\"]\n```"},
		"slice": {Name: "slice", Signature: "slice x i [j]", Doc: "Returns a slice of the first argument. With two args, equivalent to `x[i:]`. With three args, equivalent to `x[i:j]`.\n\n**Example:**\n```\n{{slice .Items 1}}     → .Items[1:]\n{{slice .Items 0 3}}   → .Items[0:3]\n```"},
		"js": {Name: "js", Signature: "js args...", Doc: "Returns the escaped JavaScript equivalent of the textual representation of its arguments.\n\n**Example:**\n```\n<script>var name = \"{{js .Name}}\";</script>\n```"},
		"len": {Name: "len", Signature: "len x", Doc: "Returns the integer length of its argument (string, slice, array, map, or channel).\n\n**Example:**\n```\n{{len .Items}}\n{{if gt (len .Items) 0}}has items{{end}}\n```"},
		"not": {Name: "not", Signature: "not x", Doc: "Returns the boolean negation of its argument.\n\n**Example:**\n```\n{{if not .IsHidden}}visible{{end}}\n```"},
		"or": {Name: "or", Signature: "or x y ...", Doc: "Returns the first non-empty argument or the last argument. Useful for default values.\n\n**Example:**\n```\n{{or .Title \"Untitled\"}}\n{{if or .IsAdmin .IsMod}}...{{end}}\n```"},
		"print": {Name: "print", Signature: "print args...", Doc: "An alias for `fmt.Sprint`. Concatenates arguments with default formatting.\n\n**Example:**\n```\n{{print \"Hello, \" .Name}}\n```"},
		"printf": {Name: "printf", Signature: "printf format args...", Doc: "An alias for `fmt.Sprintf`. Formats according to the format string.\n\n**Example:**\n```\n{{printf \"Hello, %s! You have %d items.\" .Name .Count}}\n{{printf \"%.2f\" .Price}}\n```"},
		"println": {Name: "println", Signature: "println args...", Doc: "An alias for `fmt.Sprintln`. Concatenates with spaces and a trailing newline.\n\n**Example:**\n```\n{{println \"Name:\" .Name}}\n```"},
		"urlquery": {Name: "urlquery", Signature: "urlquery args...", Doc: "Returns the URL query-escaped value of the textual representation of its arguments.\n\n**Example:**\n```\n<a href=\"/search?q={{urlquery .Query}}\">Search</a>\n```"},
		"eq": {Name: "eq", Signature: "eq arg1 arg2 ...", Doc: "Returns true if arg1 equals any subsequent argument: `arg1==arg2 || arg1==arg3 ...`\n\n**Example:**\n```\n{{if eq .Status \"active\"}}...{{end}}\n{{if eq .Role \"admin\" \"superadmin\"}}...{{end}}\n```"},
		"ne": {Name: "ne", Signature: "ne arg1 arg2", Doc: "Returns true if `arg1 != arg2`.\n\n**Example:**\n```\n{{if ne .Status \"deleted\"}}...{{end}}\n```"},
		"lt": {Name: "lt", Signature: "lt arg1 arg2", Doc: "Returns true if `arg1 < arg2`.\n\n**Example:**\n```\n{{if lt .Age 18}}minor{{end}}\n```"},
		"le": {Name: "le", Signature: "le arg1 arg2", Doc: "Returns true if `arg1 <= arg2`.\n\n**Example:**\n```\n{{if le .Count 10}}few items{{end}}\n```"},
		"gt": {Name: "gt", Signature: "gt arg1 arg2", Doc: "Returns true if `arg1 > arg2`.\n\n**Example:**\n```\n{{if gt (len .Items) 0}}has items{{end}}\n```"},
		"ge": {Name: "ge", Signature: "ge arg1 arg2", Doc: "Returns true if `arg1 >= arg2`.\n\n**Example:**\n```\n{{if ge .Score 90}}excellent{{end}}\n```"},
	}
}

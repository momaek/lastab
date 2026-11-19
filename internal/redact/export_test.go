package redact

// TextAll is Text running every rule, without the literal prefilter.
func TextAll(s string) (string, bool) { return text(s, false) }

package install

import "strings"

// pathListHas says whether a PATH value of Windows, entries separated by
// semicolons, has dir, ignoring case, quotes and a trailing backslash.
// expand expands environment variables such as %LOCALAPPDATA%.
func pathListHas(list, dir string, expand func(string) string) bool {
	want := normDir(dir)
	for _, p := range strings.Split(list, ";") {
		if p = strings.TrimSpace(strings.Trim(p, `"`)); p == "" {
			continue
		}
		if expand != nil {
			p = expand(p)
		}
		if strings.EqualFold(normDir(p), want) {
			return true
		}
	}
	return false
}

// pathListAdd appends dir to a PATH value unless it has it.
func pathListAdd(list, dir string, expand func(string) string) (string, bool) {
	if pathListHas(list, dir, expand) {
		return list, false
	}
	list = strings.TrimRight(list, "; ")
	if list == "" {
		return dir, true
	}
	return list + ";" + dir, true
}

func normDir(p string) string { return strings.TrimRight(strings.TrimSpace(p), `\/`) }

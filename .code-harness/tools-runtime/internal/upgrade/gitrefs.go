package upgrade

import (
	"os/exec"
	"strings"
)

type GitRefs struct{ GitPath string }

func (g GitRefs) DetectBaseRef() (string, bool) {
	git := g.GitPath
	if git == "" { git = "git" }
	if out, err := exec.Command(git, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD").Output(); err == nil {
		if v := strings.TrimSpace(string(out)); v != "" { return v, true }
	}
	out, err := exec.Command(git, "for-each-ref", "--format=%(refname:short)", "refs/remotes/origin", "refs/heads").Output()
	if err != nil { return "", false }
	lines := strings.Fields(string(out))
	has := func(v string) bool { for _, x := range lines { if x == v { return true } }; return false }
	for _, v := range []string{"origin/master", "origin/main", "origin/develop", "master", "main", "develop"} { if has(v) { return v, true } }
	return "", false
}

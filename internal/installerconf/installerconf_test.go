package installerconf_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/webinstall/webi-installers/internal/installerconf"
)

func TestSimpleGitHub(t *testing.T) {
	c := confFromString(t, `
source = github
owner = sharkdp
repo = bat
`)
	assertEqual(t, "Source", c.Source, "github")
	assertEqual(t, "Owner", c.Owner, "sharkdp")
	assertEqual(t, "Repo", c.Repo, "bat")
	assertEqual(t, "TagPrefix", c.TagPrefix, "")
	assertEqual(t, "VersionPrefix", c.VersionPrefix, "")

	if len(c.Exclude) != 0 {
		t.Errorf("Exclude = %v, want empty", c.Exclude)
	}
}

func TestVersionPrefix(t *testing.T) {
	c := confFromString(t, `
source = github
owner = jqlang
repo = jq
version_prefix = jq-
`)
	assertEqual(t, "VersionPrefix", c.VersionPrefix, "jq-")
}

func TestExclude(t *testing.T) {
	c := confFromString(t, `
source = github
owner = gohugoio
repo = hugo
exclude = _extended_, Linux-64bit
`)
	if len(c.Exclude) != 2 {
		t.Fatalf("Exclude has %d items, want 2: %v", len(c.Exclude), c.Exclude)
	}
	assertEqual(t, "Exclude[0]", c.Exclude[0], "_extended_")
	assertEqual(t, "Exclude[1]", c.Exclude[1], "Linux-64bit")
}

func TestMonorepoTagPrefix(t *testing.T) {
	c := confFromString(t, `
source = github
owner = therootcompany
repo = golib
tag_prefix = tools/monorel/
`)
	assertEqual(t, "TagPrefix", c.TagPrefix, "tools/monorel/")
}

func TestNodeDist(t *testing.T) {
	c := confFromString(t, `
source = nodedist
url = https://nodejs.org/download/release
`)
	assertEqual(t, "Source", c.Source, "nodedist")
	assertEqual(t, "BaseURL", c.BaseURL, "https://nodejs.org/download/release")
}

func TestGiteaBaseURL(t *testing.T) {
	c := confFromString(t, `
source = gitea
base_url = https://gitea.com
owner = xorm
repo = xorm
`)
	assertEqual(t, "Source", c.Source, "gitea")
	assertEqual(t, "BaseURL", c.BaseURL, "https://gitea.com")
	assertEqual(t, "Owner", c.Owner, "xorm")
}

func TestBlanksAndComments(t *testing.T) {
	c := confFromString(t, `
# Hugo config
source = github

# owner line
owner = foo
`)
	assertEqual(t, "Source", c.Source, "github")
	assertEqual(t, "Owner", c.Owner, "foo")
}

func TestExtraKeys(t *testing.T) {
	c := confFromString(t, `
source = github
owner = foo
repo = bar
custom_thing = hello
`)
	if c.Extra == nil || c.Extra["custom_thing"] != "hello" {
		t.Errorf("Extra[custom_thing] = %q, want hello", c.Extra["custom_thing"])
	}
}

func TestEmptyExclude(t *testing.T) {
	c := confFromString(t, "source = github\n")
	if c.Exclude != nil {
		t.Errorf("Exclude = %v, want nil", c.Exclude)
	}
}

// helpers

func confFromString(t *testing.T, content string) *installerconf.Conf {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "releases.conf")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := installerconf.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func assertEqual(t *testing.T, name, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", name, got, want)
	}
}

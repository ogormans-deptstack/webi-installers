package installerconf_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/webinstall/webi-installers/internal/installerconf"
)

func TestRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "releases.conf")
	os.WriteFile(path, []byte(`
# Hugo release config
source = github
owner = gohugoio
repo = hugo
`), 0o644)

	c, err := installerconf.Read(path)
	if err != nil {
		t.Fatal(err)
	}

	if c.Source() != "github" {
		t.Errorf("Source() = %q, want github", c.Source())
	}
	if c.Get("owner") != "gohugoio" {
		t.Errorf("owner = %q, want gohugoio", c.Get("owner"))
	}
	if c.Get("repo") != "hugo" {
		t.Errorf("repo = %q, want hugo", c.Get("repo"))
	}
}

func TestReadMonorepo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "releases.conf")
	os.WriteFile(path, []byte(`source = github
owner = therootcompany
repo = golib
tag_prefix = tools/monorel/
`), 0o644)

	c, err := installerconf.Read(path)
	if err != nil {
		t.Fatal(err)
	}

	if c.Get("tag_prefix") != "tools/monorel/" {
		t.Errorf("tag_prefix = %q", c.Get("tag_prefix"))
	}
}

func TestReadNodeDist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "releases.conf")
	os.WriteFile(path, []byte(`source = nodedist
url = https://nodejs.org/download/release
`), 0o644)

	c, err := installerconf.Read(path)
	if err != nil {
		t.Fatal(err)
	}

	if c.Source() != "nodedist" {
		t.Errorf("Source() = %q, want nodedist", c.Source())
	}
	if c.Get("url") != "https://nodejs.org/download/release" {
		t.Errorf("url = %q", c.Get("url"))
	}
}

func TestReadSkipsBlanksAndComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "releases.conf")
	os.WriteFile(path, []byte(`
# comment
source = github

# another comment
owner = foo
`), 0o644)

	c, err := installerconf.Read(path)
	if err != nil {
		t.Fatal(err)
	}

	if c.Source() != "github" {
		t.Errorf("Source() = %q", c.Source())
	}
	if c.Get("owner") != "foo" {
		t.Errorf("owner = %q", c.Get("owner"))
	}
}

func TestGetMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "releases.conf")
	os.WriteFile(path, []byte("source = github\n"), 0o644)

	c, err := installerconf.Read(path)
	if err != nil {
		t.Fatal(err)
	}

	if c.Get("nonexistent") != "" {
		t.Errorf("Get(nonexistent) = %q, want empty", c.Get("nonexistent"))
	}
}

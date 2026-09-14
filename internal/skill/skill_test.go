package skill

import (
	"os"
	"path/filepath"
	"testing"
)

// bundled writes a fake plugin checkout: a skills/fleet-tasks/SKILL.md under a
// temp root, and points hostpath.Root at it.
func bundled(t *testing.T) (root, source string) {
	t.Helper()
	root = t.TempDir()
	source = filepath.Join(root, "skills", dirName)
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("---\nname: fleet-tasks\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HERDR_PLUGIN_ROOT", root)
	return root, source
}

func TestInstallLinksTheBundledSkill(t *testing.T) {
	_, source := bundled(t)
	target := t.TempDir()

	if err := Install(target); err != nil {
		t.Fatalf("install: %v", err)
	}
	got, err := os.Readlink(filepath.Join(target, dirName))
	if err != nil {
		t.Fatalf("no symlink: %v", err)
	}
	if got != source {
		t.Fatalf("link points at %q, want %q", got, source)
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	_, source := bundled(t)
	target := t.TempDir()

	if err := Install(target); err != nil {
		t.Fatal(err)
	}
	if err := Install(target); err != nil {
		t.Fatalf("second install: %v", err)
	}
	got, err := os.Readlink(filepath.Join(target, dirName))
	if err != nil || got != source {
		t.Fatalf("link after reinstall = %q, %v", got, err)
	}
}

func TestInstallRepointsAStaleLink(t *testing.T) {
	_, source := bundled(t)
	target := t.TempDir()
	link := filepath.Join(target, dirName)

	if err := os.Symlink(filepath.Join(target, "somewhere-else"), link); err != nil {
		t.Fatal(err)
	}
	if err := Install(target); err != nil {
		t.Fatalf("install over a stale link: %v", err)
	}
	got, err := os.Readlink(link)
	if err != nil || got != source {
		t.Fatalf("link after repoint = %q, %v", got, err)
	}
}

func TestInstallRefusesToClobberRealFiles(t *testing.T) {
	bundled(t)
	target := t.TempDir()
	link := filepath.Join(target, dirName)

	if err := os.Mkdir(link, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Install(target); err == nil {
		t.Fatal("want an error when the link path is a real directory")
	}
	if info, err := os.Lstat(link); err != nil || !info.IsDir() {
		t.Fatalf("the real directory was touched: %v %v", info, err)
	}
}

func TestInstallErrorsWhenTheSkillIsMissing(t *testing.T) {
	t.Setenv("HERDR_PLUGIN_ROOT", t.TempDir()) // no skills/ in it
	if err := Install(t.TempDir()); err == nil {
		t.Fatal("want an error when the bundled skill is absent")
	}
}

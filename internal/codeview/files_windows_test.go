//go:build windows

package codeview

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func makeTestFIFO(t *testing.T, path string) { t.Helper(); t.Fatal("FIFO fixture is Unix-only") }

func TestWindowsNodePathAliases(t *testing.T) {
	for _, p := range []string{`C:/secret`, `C:secret`, `\\server\share`, `dir\file`, `../secret`, `.git/config`, `.GiT/config`, `file:stream`, `file.`, `file `, `dir /file`, `NUL`, `nul.txt`, `COM1`, `LPT9.log`, `CONIN$`} {
		if _, err := nodePath(nodeID(p)); Status(err) != 400 {
			t.Errorf("accepted %q: %v", p, err)
		}
	}
	for _, p := range []string{"", "normal.txt", "目录/文件.txt", ".github/workflows/test.yml"} {
		if _, err := nodePath(nodeID(p)); err != nil {
			t.Errorf("rejected %q: %v", p, err)
		}
	}
}

func windowsBinding(t *testing.T) *binding {
	t.Helper()
	root := t.TempDir()
	info, err := identityInfo(root)
	if err != nil {
		t.Fatal(err)
	}
	return &binding{Context: Context{Root: root}, identity: info}
}

func TestWindowsHandleTraversalAndBoundaries(t *testing.T) {
	b := windowsBinding(t)
	if err := os.Mkdir(filepath.Join(b.Root, "sub"), 0700); err != nil {
		t.Fatal(err)
	}
	write(t, b.Root, "sub/file.txt", "hello\n")
	doc, err := b.read("sub/file.txt")
	if err != nil || doc.Text != "hello\n" {
		t.Fatalf("read: %+v %v", doc, err)
	}
	if err := os.MkdirAll(filepath.Join(b.Root, "nested", ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(b.Root, "bare", "objects"), 0700); err != nil {
		t.Fatal(err)
	}
	write(t, b.Root, "bare/HEAD", "ref: refs/heads/main\n")
	for _, p := range []string{"nested", "nested/.git/config", "bare", "bare/HEAD"} {
		if f, e := b.open(p); e == nil {
			f.Close()
			t.Errorf("opened boundary %s", p)
		}
	}
	listing, err := b.listing(context.Background(), "", false)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, n := range listing.Children {
		kinds[n.Name] = n.Kind
	}
	if kinds["sub"] != "directory" || kinds["nested"] != "boundary" || kinds["bare"] != "boundary" {
		t.Fatalf("kinds: %v", kinds)
	}
	// An alternate data stream must never be addressable by a node ID.
	if _, err := b.read("sub/file.txt:stream"); Status(err) != 400 {
		t.Fatalf("stream accepted: %v", err)
	}
}

func TestWindowsJunctionNeverFollowed(t *testing.T) {
	b := windowsBinding(t)
	outside := t.TempDir()
	write(t, outside, "secret.txt", "not visible")
	junction := filepath.Join(b.Root, "junction")
	// Directory junctions require no Developer Mode or symlink privilege.
	if out, err := exec.Command("cmd.exe", "/D", "/C", "mklink", "/J", junction, outside).CombinedOutput(); err != nil {
		t.Fatalf("create junction: %s: %v", out, err)
	}
	defer os.Remove(junction)
	for _, p := range []string{"junction", "junction/secret.txt"} {
		if f, e := b.open(p); e == nil {
			f.Close()
			t.Errorf("followed junction %q", p)
		}
	}
	listing, err := b.listing(context.Background(), "", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(listing.Children) != 1 || listing.Children[0].Kind != "symlink" {
		t.Fatalf("junction listing: %+v", listing)
	}
}

func TestWindowsRootReplacementRejected(t *testing.T) {
	b := windowsBinding(t)
	old := b.Root + "-old"
	if err := os.Rename(b.Root, old); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(old)
	if err := os.Mkdir(b.Root, 0700); err != nil {
		t.Fatal(err)
	}
	if f, err := b.open(""); Status(err) != 409 {
		if f != nil {
			f.Close()
		}
		t.Fatalf("replacement accepted: %v", err)
	}
}

func TestWindowsRelativeOpenPinsDirectory(t *testing.T) {
	b := windowsBinding(t)
	write(t, b.Root, "file.txt", "original")
	f, err := b.open("")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	old := b.Root + "-moved"
	if err = os.Rename(b.Root, old); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(old)
	if err = os.Mkdir(b.Root, 0700); err != nil {
		t.Fatal(err)
	}
	write(t, b.Root, "file.txt", "replacement")
	child, err := openRelative(f, "file.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer child.Close()
	data := make([]byte, 8)
	n, err := child.Read(data)
	if err != nil || string(data[:n]) != "original" {
		t.Fatalf("opened replacement: %q %v", data[:n], err)
	}
}

func TestWindowsGitDirectoryReplacementRejected(t *testing.T) {
	s, id, root := setup(t)
	gitTest(t, root, "init", "-q")
	if _, err := s.Context(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(root, ".git"), filepath.Join(root, "old-git")); err != nil {
		t.Fatal(err)
	}
	gitTest(t, root, "init", "-q")
	if _, err := s.Context(context.Background(), id); Status(err) != 409 {
		t.Fatalf("replacement Git identity accepted: %v", err)
	}
}

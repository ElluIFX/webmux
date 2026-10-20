package api

import (
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"testing"
	"unsafe"
)

func TestUploadRejectsWindowsAliasesAndStreams(t *testing.T) {
	for _, name := range []string{"CON", "con.txt", "CON .txt", "nul.tar.gz", "AUX", "PRN", "COM1.log", "LPT9", "COM¹", "a:stream", "file.", "file ", `a"b`, "a?b", "a*b", "CONIN$", "CONOUT$"} {
		if got, err := uploadFilename(name); err == nil {
			t.Errorf("unsafe name %q accepted as %q", name, got)
		}
	}
	for _, name := range []string{"report.txt", "文件.txt", "COM10.txt", "a'b.txt"} {
		if _, err := uploadFilename(name); err != nil {
			t.Errorf("safe name %q: %v", name, err)
		}
	}
}

func TestUploadDirectoryAndFilesHavePrivateWindowsACL(t *testing.T) {
	root := t.TempDir()
	t.Setenv("TMP", root)
	t.Setenv("TEMP", root)
	dir, err := privateUploadDir()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "upload.txt")
	if err := os.WriteFile(file, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{dir, file} {
		sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			t.Fatal(err)
		}
		if path == dir {
			control, _, err := sd.Control()
			if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
				t.Fatalf("directory DACL is not protected: %v", err)
			}
		}
		acl, _, err := sd.DACL()
		if err != nil || acl == nil || acl.AceCount != 1 {
			t.Fatalf("unexpected DACL for %s: %s (%v)", path, sd.String(), err)
		}
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(acl, 0, &ace); err != nil {
			t.Fatal(err)
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || !sid.Equals(user.User.Sid) {
			t.Fatalf("upload access granted to another identity: %s", sd.String())
		}
	}
}

func TestUploadDirectoryWithExtendedLengthTemp(t *testing.T) {
	root := t.TempDir()
	extended := `\\?\` + root
	t.Setenv("TMP", extended)
	t.Setenv("TEMP", extended)
	dir, err := privateUploadDir()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	file := filepath.Join(dir, "upload.txt")
	if err := os.WriteFile(file, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(file); err != nil || string(data) != "private" {
		t.Fatalf("extended TEMP upload: %q, %v", data, err)
	}
}

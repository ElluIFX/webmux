//go:build windows

package codeview

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Node IDs use slash-separated relative paths on every platform. Reject Win32
// aliases, streams, device names, and normalization before reaching NT APIs.
func validPlatformPath(p string) bool {
	if strings.ContainsAny(p, `\:`) {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if strings.TrimRight(part, " .") != part {
			return false
		}
		for _, c := range part {
			if c < 32 {
				return false
			}
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		switch base {
		case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
			return false
		}
		if (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && strings.ContainsAny(strings.TrimPrefix(strings.TrimPrefix(base, "COM"), "LPT"), "¹²³") {
			return false
		}
		if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9' {
			return false
		}
		if strings.ContainsAny(part, "\x00<>\"|?*") {
			return false
		}
	}
	return true
}

// openRelative opens exactly one component relative to a pinned directory
// handle. FILE_OPEN_REPARSE_POINT prevents junctions, symlinks and other reparse
// providers from being followed; no path-based check/open race is involved.
func openRelative(parent *os.File, name string) (*os.File, error) {
	str, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return nil, err
	}
	oa := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(parent.Fd()), ObjectName: str, Attributes: windows.OBJ_CASE_INSENSITIVE}
	oa.Length = uint32(unsafe.Sizeof(oa))
	var handle windows.Handle
	var iosb windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&handle, windows.FILE_GENERIC_READ, &oa, &iosb, nil, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_OPEN, windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	if err != nil {
		var status windows.NTStatus
		if errors.As(err, &status) {
			err = status.Errno()
		}
		return nil, &os.PathError{Op: "open", Path: name, Err: err}
	}
	return os.NewFile(uintptr(handle), filepath.Join(parent.Name(), name)), nil
}
func handleInfo(f *os.File) (windows.ByHandleFileInformation, error) {
	var info windows.ByHandleFileInformation
	err := windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &info)
	return info, err
}
func boundaryWindows(f *os.File) bool {
	exists := func(name string) bool {
		child, err := openRelative(f, name)
		if err != nil {
			return !os.IsNotExist(err)
		}
		child.Close()
		return true
	}
	return exists(".git") || (exists("HEAD") && exists("objects"))
}
func checkWindowsFile(f *os.File) error {
	info, err := handleInfo(f)
	if err != nil {
		return err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return problem(400, "不支持读取重解析点或链接")
	}
	typ, err := windows.GetFileType(windows.Handle(f.Fd()))
	if err != nil {
		return err
	}
	if typ != windows.FILE_TYPE_DISK {
		return problem(400, "不支持读取特殊文件")
	}
	// Check the normalized on-disk name as well, so DOS 8.3 aliases cannot
	// expose a .git directory that the user-supplied name did not identify.
	buf := make([]uint16, 32768)
	n, err := windows.GetFinalPathNameByHandle(windows.Handle(f.Fd()), &buf[0], uint32(len(buf)), 0)
	if err != nil || n >= uint32(len(buf)) {
		return problem(400, "无法验证文件路径")
	}
	for _, part := range strings.Split(windows.UTF16ToString(buf[:n]), `\`) {
		if strings.EqualFold(part, ".git") {
			return problem(400, "Git 管理路径不可读取")
		}
	}
	return nil
}
func (b *binding) open(p string) (*os.File, error) {
	if _, err := nodePath(nodeID(p)); err != nil {
		return nil, err
	}
	path, err := windows.UTF16PtrFromString(b.Root)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(path, windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(handle), b.Root)
	fail := func(err error) (*os.File, error) { f.Close(); return nil, err }
	if err = checkWindowsFile(f); err != nil {
		return fail(err)
	}
	stat, err := f.Stat()
	if err != nil || !stat.IsDir() || !os.SameFile(stat, b.identity) {
		return fail(problem(409, "浏览根已替换"))
	}
	if p == "" {
		return f, nil
	}
	for _, part := range strings.Split(p, "/") {
		next, e := openRelative(f, part)
		f.Close()
		if e != nil {
			if os.IsNotExist(e) {
				return nil, os.ErrNotExist
			}
			return nil, problem(400, "路径不可安全读取: %v", e)
		}
		f = next
		if e = checkWindowsFile(f); e != nil {
			return fail(e)
		}
		stat, e = f.Stat()
		if e != nil {
			return fail(e)
		}
		if (b.gitIdentity != nil && os.SameFile(stat, b.gitIdentity)) || (b.commonIdentity != nil && os.SameFile(stat, b.commonIdentity)) {
			return fail(problem(400, "Git 管理路径不可读取"))
		}
		if stat.IsDir() && boundaryWindows(f) {
			return fail(problem(400, "嵌套仓库边界不可遍历"))
		}
		if !stat.IsDir() && !stat.Mode().IsRegular() {
			return fail(problem(400, "不支持读取特殊文件"))
		}
	}
	return f, nil
}
func entryDetails(parent *os.File, name string, entry os.DirEntry) (kind, link string, err error) {
	f, err := openRelative(parent, name)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	info, err := handleInfo(f)
	if err != nil {
		return "", "", err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return "symlink", "", nil
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 {
		if boundaryWindows(f) {
			return "boundary", "", nil
		}
		return "directory", "", nil
	}
	return "file", "", nil
}

// os.Stat on Windows can defer reading the file ID until os.SameFile is
// called. Capture it from an open handle now, before the path can be replaced.
func identityInfo(path string) (os.FileInfo, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(handle), path)
	defer f.Close()
	info, err := handleInfo(f)
	if err != nil {
		return nil, err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return nil, problem(409, "身份路径已变为重解析点")
	}
	return f.Stat()
}

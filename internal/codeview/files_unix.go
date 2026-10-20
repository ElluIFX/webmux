//go:build !windows

package codeview

import (
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"strings"
)

func validPlatformPath(string) bool { return true }

// Open each segment relative to an already opened directory. O_NOFOLLOW and
// O_NONBLOCK prevent symlink races and blocking special-file opens on Unix.
func boundary(fd int) bool {
	var stat unix.Stat_t
	return unix.Fstatat(fd, ".git", &stat, unix.AT_SYMLINK_NOFOLLOW) == nil || (unix.Fstatat(fd, "HEAD", &stat, unix.AT_SYMLINK_NOFOLLOW) == nil && unix.Fstatat(fd, "objects", &stat, unix.AT_SYMLINK_NOFOLLOW) == nil)
}
func (b *binding) open(p string) (*os.File, error) {
	if _, err := nodePath(nodeID(p)); err != nil {
		return nil, err
	}
	full := filepath.Join(b.Root, p)
	for _, g := range b.gitDirs {
		if full == g || strings.HasPrefix(full, g+"/") {
			return nil, problem(400, "Git 管理路径不可读取")
		}
	}
	fd, err := unix.Open(b.Root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), b.Root)
	stat, err := f.Stat()
	if err != nil || !os.SameFile(stat, b.identity) {
		f.Close()
		return nil, problem(409, "浏览根已替换")
	}
	if p == "" {
		return f, nil
	}
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if i > 0 {
			if boundary(int(f.Fd())) {
				f.Close()
				return nil, problem(400, "嵌套仓库边界不可遍历")
			}
		}
		flags := unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK | unix.O_CLOEXEC
		if i < len(parts)-1 {
			flags |= unix.O_DIRECTORY
		}
		next, e := unix.Openat(int(f.Fd()), part, flags, 0)
		f.Close()
		if e != nil {
			if os.IsNotExist(e) {
				return nil, os.ErrNotExist
			}
			return nil, problem(400, "路径不可安全读取: %v", e)
		}
		f = os.NewFile(uintptr(next), full)
	}
	stat, err = f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if stat.IsDir() && p != "" {
		if boundary(int(f.Fd())) {
			f.Close()
			return nil, problem(400, "嵌套仓库边界不可遍历")
		}
	}
	if !stat.IsDir() && !stat.Mode().IsRegular() {
		f.Close()
		return nil, problem(400, "不支持读取特殊文件或链接")
	}
	return f, nil
}
func entryDetails(parent *os.File, name string, entry os.DirEntry) (kind, link string, err error) {
	kind = "file"
	if entry.IsDir() {
		kind = "directory"
		fd, e := unix.Openat(int(parent.Fd()), name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if e != nil {
			return "", "", e
		}
		if boundary(fd) {
			kind = "boundary"
		}
		unix.Close(fd)
	} else if entry.Type()&os.ModeSymlink != 0 {
		kind = "symlink"
		data := make([]byte, 4096)
		if count, e := unix.Readlinkat(int(parent.Fd()), name, data); e == nil {
			link = string(data[:count])
		}
	} else if !entry.Type().IsRegular() {
		kind = "special"
	}
	return kind, link, nil
}

package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Chmod on Windows only changes the read-only attribute. Create the directory
// with a protected DACL from the outset, even if TEMP is shared or permissive.
// OI/CI make files and subdirectories inherit the same user-only access.
func privateUploadDir() (string, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", err
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		return "", err
	}
	defer runtime.KeepAlive(sd)
	sa := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	base, err := filepath.Abs(os.TempDir())
	if err != nil {
		return "", err
	}
	for range 10 {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return "", err
		}
		path := filepath.Join(base, "webmux-upload-"+hex.EncodeToString(random[:]))
		native := path
		if strings.HasPrefix(native, `\\?\`) {
			// Already an extended-length DOS or UNC path.
		} else if strings.HasPrefix(native, `\\`) {
			native = `\\?\UNC\` + native[2:]
		} else {
			native = `\\?\` + native
		}
		ptr, err := windows.UTF16PtrFromString(native)
		if err != nil {
			return "", err
		}
		if err = windows.CreateDirectory(ptr, &sa); err == nil {
			return path, nil
		} else if err != windows.ERROR_ALREADY_EXISTS {
			return "", err
		}
	}
	return "", fmt.Errorf("could not allocate a unique upload directory")
}

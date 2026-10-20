package workspace

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Start suspended so no Git hook or credential helper can escape the job
// between process creation and assignment. Closing the job kills descendants,
// including those holding pipes after the Git leader exits.
func runGitCommand(ctx context.Context, cmd *exec.Cmd) error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(job)
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED | windows.CREATE_NO_WINDOW}
	var mu sync.Mutex
	assigned := false
	cmd.Cancel = func() error {
		mu.Lock()
		defer mu.Unlock()
		if assigned {
			return windows.TerminateJobObject(job, 1)
		}
		return cmd.Process.Kill()
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	mu.Lock()
	process, openErr := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if openErr == nil {
		openErr = windows.AssignProcessToJobObject(job, process)
		windows.CloseHandle(process)
	}
	if openErr == nil {
		assigned = true
		openErr = resumeGitProcess(uint32(cmd.Process.Pid))
	}
	if openErr != nil {
		_ = cmd.Process.Kill()
	}
	mu.Unlock()
	waitErr := cmd.Wait()
	if openErr != nil {
		return fmt.Errorf("contain Git process: %w", openErr)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return waitErr
}

func resumeGitProcess(pid uint32) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		thread, e := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if e != nil {
			return e
		}
		_, e = windows.ResumeThread(thread)
		windows.CloseHandle(thread)
		return e
	}
	if err != nil && err != windows.ERROR_NO_MORE_FILES {
		return err
	}
	return os.ErrProcessDone
}

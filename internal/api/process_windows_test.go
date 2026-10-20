package api

import "golang.org/x/sys/windows"

func rawTestInput() {
	handle, err := windows.GetStdHandle(windows.STD_INPUT_HANDLE)
	if err == nil {
		_ = windows.SetConsoleMode(handle, windows.ENABLE_VIRTUAL_TERMINAL_INPUT)
	}
}

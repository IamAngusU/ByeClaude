package metrics

import (
	"errors"
	"fmt"
	"syscall"
)

// An exclusive Windows handle is released by the OS even if the process dies.
// Keep the lock file in place: unlinking live lock files would break exclusion.
func tryLock(path string) (func(), error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil,
		syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if errors.Is(err, syscall.Errno(32)) {
		return nil, errBusy
	} // ERROR_SHARING_VIOLATION
	if err != nil {
		return nil, err
	}
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(h, &info); err != nil {
		_ = syscall.CloseHandle(h)
		return nil, err
	}
	if info.FileAttributes&(syscall.FILE_ATTRIBUTE_REPARSE_POINT|syscall.FILE_ATTRIBUTE_DIRECTORY) != 0 {
		_ = syscall.CloseHandle(h)
		return nil, fmt.Errorf("metrics lock is not a regular file")
	}
	return func() { _ = syscall.CloseHandle(h) }, nil
}

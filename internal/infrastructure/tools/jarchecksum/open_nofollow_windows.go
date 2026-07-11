//go:build windows

package jarchecksum

import (
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

type fileAttributeTagInfo struct {
	FileAttributes uint32
	ReparseTag     uint32
}

// openFileNoFollow opens the final path entry itself and rejects every reparse point.
func openFileNoFollow(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(
		name,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_SEQUENTIAL_SCAN,
		0,
	)
	if err != nil {
		return nil, err
	}
	var info fileAttributeTagInfo
	if err := windows.GetFileInformationByHandleEx(
		h,
		windows.FileAttributeTagInfo,
		(*byte)(unsafe.Pointer(&info)), //nolint:gosec // Win32 requires a FILE_ATTRIBUTE_TAG_INFO output buffer
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(h)
		return nil, err
	}
	if info.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DIRECTORY) != 0 {
		_ = windows.CloseHandle(h)
		return nil, os.ErrInvalid
	}
	f := os.NewFile(uintptr(h), path)
	if f == nil {
		_ = windows.CloseHandle(h)
		return nil, os.ErrInvalid
	}
	return f, nil
}

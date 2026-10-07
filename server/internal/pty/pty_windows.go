//go:build windows
// +build windows

package pty

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modkernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procCreatePseudoConsole         = modkernel32.NewProc("CreatePseudoConsole")
	procResizePseudoConsole         = modkernel32.NewProc("ResizePseudoConsole")
	procClosePseudoConsole          = modkernel32.NewProc("ClosePseudoConsole")
	procInitializeProcThreadAttrLst = modkernel32.NewProc("InitializeProcThreadAttributeList")
	procUpdateProcThreadAttr        = modkernel32.NewProc("UpdateProcThreadAttribute")
	procDeleteProcThreadAttrLst     = modkernel32.NewProc("DeleteProcThreadAttributeList")
)

const (
	PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE = 0x00020016
)

type coord struct {
	X int16
	Y int16
}

type startupInfoEx struct {
	windows.StartupInfo
	lpAttributeList *byte
}

type WindowsPTY struct {
	hPC           windows.Handle
	processHandle windows.Handle
	threadHandle  windows.Handle
	processID     int

	inPipeWrite  *os.File
	outPipeRead  *os.File

	mu     sync.Mutex
	closed bool
}

func Start(command string, dir string, cols, rows uint16) (*WindowsPTY, error) {
	if cols == 0 {
		cols = 120
	}
	if rows == 0 {
		rows = 30
	}

	// 1. Create communication pipes
	var inPipeRead, inPipeWrite windows.Handle
	if err := windows.CreatePipe(&inPipeRead, &inPipeWrite, nil, 0); err != nil {
		return nil, fmt.Errorf("failed to create input pipe: %w", err)
	}

	var outPipeRead, outPipeWrite windows.Handle
	if err := windows.CreatePipe(&outPipeRead, &outPipeWrite, nil, 0); err != nil {
		windows.CloseHandle(inPipeRead)
		windows.CloseHandle(inPipeWrite)
		return nil, fmt.Errorf("failed to create output pipe: %w", err)
	}

	// 2. Create PseudoConsole
	var hPC windows.Handle
	coordVal := uintptr(uint32(cols) | (uint32(rows) << 16))
	r0, _, _ := procCreatePseudoConsole.Call(
		coordVal,
		uintptr(inPipeRead),
		uintptr(outPipeWrite),
		0,
		uintptr(unsafe.Pointer(&hPC)),
	)
	if int32(r0) < 0 { // HRESULT failure
		windows.CloseHandle(inPipeRead)
		windows.CloseHandle(inPipeWrite)
		windows.CloseHandle(outPipeRead)
		windows.CloseHandle(outPipeWrite)
		return nil, fmt.Errorf("CreatePseudoConsole failed with HRESULT: 0x%X", uint32(r0))
	}

	// PseudoConsole owns inPipeRead and outPipeWrite now; close our duplicate handles
	windows.CloseHandle(inPipeRead)
	windows.CloseHandle(outPipeWrite)

	// 3. Initialize Thread Attribute List for ConPTY
	var attrSize uintptr
	procInitializeProcThreadAttrLst.Call(0, 1, 0, uintptr(unsafe.Pointer(&attrSize)))
	if attrSize == 0 {
		procClosePseudoConsole.Call(uintptr(hPC))
		windows.CloseHandle(inPipeWrite)
		windows.CloseHandle(outPipeRead)
		return nil, errors.New("failed to calculate ProcThreadAttributeList size")
	}

	attrList := make([]byte, attrSize)
	lpAttrList := &attrList[0]
	r1, _, err := procInitializeProcThreadAttrLst.Call(
		uintptr(unsafe.Pointer(lpAttrList)),
		1,
		0,
		uintptr(unsafe.Pointer(&attrSize)),
	)
	if r1 == 0 {
		procClosePseudoConsole.Call(uintptr(hPC))
		windows.CloseHandle(inPipeWrite)
		windows.CloseHandle(outPipeRead)
		return nil, fmt.Errorf("InitializeProcThreadAttributeList failed: %w", err)
	}
	defer procDeleteProcThreadAttrLst.Call(uintptr(unsafe.Pointer(lpAttrList)))

	r1, _, err = procUpdateProcThreadAttr.Call(
		uintptr(unsafe.Pointer(lpAttrList)),
		0,
		PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE,
		uintptr(hPC),
		unsafe.Sizeof(hPC),
		0,
		0,
	)
	if r1 == 0 {
		procClosePseudoConsole.Call(uintptr(hPC))
		windows.CloseHandle(inPipeWrite)
		windows.CloseHandle(outPipeRead)
		return nil, fmt.Errorf("UpdateProcThreadAttribute failed: %w", err)
	}

	// 4. Spawn Process with ConPTY
	desktopUtf16, _ := windows.UTF16PtrFromString(`winsta0\default`)
	siEx := startupInfoEx{
		StartupInfo: windows.StartupInfo{
			Cb:         uint32(unsafe.Sizeof(startupInfoEx{})),
			Desktop:    desktopUtf16,
			Flags:      windows.STARTF_USESTDHANDLES | windows.STARTF_USESHOWWINDOW,
			ShowWindow: windows.SW_SHOWNORMAL,
		},
		lpAttributeList: lpAttrList,
	}

	var pi windows.ProcessInformation

	cmdLineUtf16, err := windows.UTF16PtrFromString(command)
	if err != nil {
		procClosePseudoConsole.Call(uintptr(hPC))
		windows.CloseHandle(inPipeWrite)
		windows.CloseHandle(outPipeRead)
		return nil, err
	}

	var dirUtf16 *uint16
	if dir != "" {
		dirUtf16, err = windows.UTF16PtrFromString(dir)
		if err != nil {
			procClosePseudoConsole.Call(uintptr(hPC))
			windows.CloseHandle(inPipeWrite)
			windows.CloseHandle(outPipeRead)
			return nil, err
		}
	}

	creationFlags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT)
	err = windows.CreateProcess(
		nil,
		cmdLineUtf16,
		nil,
		nil,
		false,
		creationFlags,
		nil,
		dirUtf16,
		&siEx.StartupInfo,
		&pi,
	)
	if err != nil {
		procClosePseudoConsole.Call(uintptr(hPC))
		windows.CloseHandle(inPipeWrite)
		windows.CloseHandle(outPipeRead)
		return nil, fmt.Errorf("CreateProcess failed: %w", err)
	}

	return &WindowsPTY{
		hPC:           hPC,
		processHandle: pi.Process,
		threadHandle:  pi.Thread,
		processID:     int(pi.ProcessId),
		inPipeWrite:   os.NewFile(uintptr(inPipeWrite), "pty-in"),
		outPipeRead:   os.NewFile(uintptr(outPipeRead), "pty-out"),
	}, nil
}

func (p *WindowsPTY) Read(b []byte) (int, error) {
	return p.outPipeRead.Read(b)
}

func (p *WindowsPTY) Write(b []byte) (int, error) {
	return p.inPipeWrite.Write(b)
}

func (p *WindowsPTY) Resize(cols, rows uint16) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return errors.New("pty already closed")
	}

	coordVal := uintptr(uint32(cols) | (uint32(rows) << 16))
	r0, _, _ := procResizePseudoConsole.Call(
		uintptr(p.hPC),
		coordVal,
	)
	if int32(r0) < 0 {
		return fmt.Errorf("ResizePseudoConsole failed with HRESULT: 0x%X", uint32(r0))
	}
	return nil
}

func (p *WindowsPTY) ProcessID() int {
	return p.processID
}

func (p *WindowsPTY) Wait() (int, error) {
	event, err := windows.WaitForSingleObject(p.processHandle, windows.INFINITE)
	if err != nil {
		return -1, err
	}
	if event == windows.WAIT_OBJECT_0 {
		var exitCode uint32
		if err := windows.GetExitCodeProcess(p.processHandle, &exitCode); err != nil {
			return -1, err
		}
		return int(exitCode), nil
	}
	return -1, errors.New("unexpected wait object return code")
}

func (p *WindowsPTY) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true

	if p.inPipeWrite != nil {
		_ = p.inPipeWrite.Close()
	}
	if p.outPipeRead != nil {
		_ = p.outPipeRead.Close()
	}
	if p.hPC != 0 {
		procClosePseudoConsole.Call(uintptr(p.hPC))
	}

	// Close process & thread handles
	if p.threadHandle != 0 {
		_ = windows.CloseHandle(p.threadHandle)
	}
	if p.processHandle != 0 {
		_ = windows.CloseHandle(p.processHandle)
	}

	return nil
}

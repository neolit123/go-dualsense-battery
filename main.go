// SPDX-License-Identifier: Apache-2.0
// Copyright 2025 the go-dualsense-battery authors

package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"syscall"
	"time"

	"github.com/getlantern/systray"
	"github.com/sstallion/go-hid"
	"golang.org/x/sys/windows"
)

const (
	appName = "go-dualsense-battery"

	vendorID  = 0x054C
	productId = 0x0CE6

	offsetBatteryUSB = 53
	offsetBatteryBT  = 54
	battery0Mask     = 0xf

	attachParentProcess = ^uint32(0) // ATTACH_PARENT_PROCESS (DWORD)-1
)

var (
	version string

	//go:embed assets/not_connected.ico
	notConnected []byte

	//go:embed assets/charging_0.ico
	charging0 []byte
	//go:embed assets/charging_1.ico
	charging1 []byte
	//go:embed assets/charging_2.ico
	charging2 []byte
	//go:embed assets/charging_3.ico
	charging3 []byte
	//go:embed assets/charging_4.ico
	charging4 []byte

	//go:embed assets/not_charging_0.ico
	notCharging0 []byte
	//go:embed assets/not_charging_1.ico
	notCharging1 []byte
	//go:embed assets/not_charging_2.ico
	notCharging2 []byte
	//go:embed assets/not_charging_3.ico
	notCharging3 []byte
	//go:embed assets/not_charging_4.ico
	notCharging4 []byte

	charging [][]byte = [][]byte{
		charging0,
		charging1,
		charging2,
		charging3,
		charging4,
	}

	notCharging [][]byte = [][]byte{
		notCharging0,
		notCharging1,
		notCharging2,
		notCharging3,
		notCharging4,
	}

	k32            = windows.NewLazyDLL("kernel32.dll")
	pAttachConsole = k32.NewProc("AttachConsole")
	pAllocConsole  = k32.NewProc("AllocConsole")
	pFreeConsole   = k32.NewProc("FreeConsole")

	flagConsole bool
)

// https://stackoverflow.com/questions/23743217/printing-output-to-a-command-window-when-golang-application-is-compiled-with-ld
func attachConsole() bool {
	var allocated = false

	// Attach a console to the process. Allocate it if needed first.
	r0, _, _ := pAttachConsole.Call(uintptr(attachParentProcess))
	if r0 == 0 {
		r0, _, _ := pAllocConsole.Call()
		if r0 != 0 {
			allocated = true
		}
	}

	// Redirect the standard streams.
	hout, err := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	if err == nil {
		os.Stdout = os.NewFile(uintptr(hout), "/dev/stdout")
	}
	herr, err := syscall.GetStdHandle(syscall.STD_ERROR_HANDLE)
	if err == nil {
		os.Stderr = os.NewFile(uintptr(herr), "/dev/stderr")
	}

	return allocated
}

func main() {
	flag.BoolVar(&flagConsole, "console", false, "Attach a debug console to the program.")
	flag.Parse()

	if flagConsole {
		if freeConsole := attachConsole(); freeConsole {
			defer pFreeConsole.Call()
		}
	}

	log.SetFlags(log.LstdFlags)
	log.SetOutput(os.Stdout)
	fmt.Fprintln(os.Stdout, "")

	onExit := func() {}
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(notConnected)
	systray.SetTitle(appName)

	menuItemAppName := systray.AddMenuItem(fmt.Sprintf("%s %s", appName, version), "")
	menuItemAppName.Disable()

	menuItemExit := systray.AddMenuItem("Exit", "Exit")
	go func() {
		<-menuItemExit.ClickedCh
		hid.Exit()
		systray.Quit()
	}()

	if err := hid.Init(); err != nil {
		setStatus(fmt.Sprintf("hid.Init error: %v", err.Error()))
		time.Sleep(2 * time.Second)
		systray.Quit()
	}

open:
	time.Sleep(1 * time.Second)

	log.Println("Opening controller device...")

	time.Sleep(1 * time.Second)

	d, err := hid.OpenFirst(vendorID, productId)
	if err != nil {
		setStatus(fmt.Sprintf("OpenFirst error: %v", err.Error()))
		systray.SetIcon(notConnected)
		goto open
	}

	log.Println("Reading device info...")

	time.Sleep(1 * time.Second)

	info, err := d.GetDeviceInfo()
	if err != nil {
		setStatus(fmt.Sprintf("GetDeviceInfo error: %v", err.Error()))
		systray.SetIcon(notConnected)
		_ = d.Close()
		goto open
	}

	var offset int
	switch info.BusType {
	case hid.BusUSB:
		offset = offsetBatteryUSB
	case hid.BusBluetooth:
		offset = offsetBatteryBT
	default:
		setStatus(fmt.Sprintf("error: unsupported BusType: %s", info.BusType))
		systray.SetIcon(notConnected)
		_ = d.Close()
		goto open
	}

	err = d.SetNonblock(true)
	if err != nil {
		setStatus(fmt.Sprintf("SetNonblock error: %v", err.Error()))
		_ = d.Close()
		goto open
	}

	log.Println("Reading status...")

	b := make([]byte, 128)
	_, err = d.Read(b)
	if err != nil {
		setStatus(fmt.Sprintf("Read error: %v", err.Error()))
		systray.SetIcon(notConnected)
		_ = d.Close()
		goto open
	}

	// Offsets and calculation referenced from:
	// - https://github.com/Ohjurot/DualSense-Windows
	battery0 := b[offset] & battery0Mask
	log.Printf("Read battery0 value of: %x", battery0)
	percent, iconIndex := battery0ToPercentAndIndex(battery0)

	setStatus(fmt.Sprintf("Connection: %s, Battery: %d%%", info.BusType, percent))

	if info.BusType == hid.BusUSB {
		systray.SetIcon(charging[iconIndex])
	} else {
		systray.SetIcon(notCharging[iconIndex])
	}

	time.Sleep(time.Second * 1)
	_ = d.Close()
	goto open
}

func setStatus(status string) {
	log.Println(status)
	systray.SetTooltip(status)
}

// battery0 has the range of 0 - 0x0f, so 16 values.
// Map that to percentage and icon index.
// The 101.0 trick for nIcons avoids branching to fit the
// iconIndex in len(charging)-1, when percent == 100.0.
func battery0ToPercentAndIndex(battery0 byte) (int, int) {
	percent := float64(battery0) / float64(battery0Mask) * 100.0
	nIcons := 101.0 / float64(len(charging))
	iconIndex := math.Floor(percent / nIcons)
	return int(percent), int(iconIndex)
}

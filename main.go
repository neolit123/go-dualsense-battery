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

	// https://controllers.fandom.com/wiki/Sony_DualSense
	vendorID         = 0x054C
	productId        = 0x0CE6
	offsetBatteryUSB = 53
	offsetBatteryBT  = 54
	battery0Mask     = 0x0A
	wakeupByte       = 0x05

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

	consoleEnabled bool
)

// https://stackoverflow.com/questions/23743217/printing-output-to-a-command-window-when-golang-application-is-compiled-with-ld
func attachConsole() {
	// Attach a console to the process. Allocate it if needed first.
	r0, _, _ := pAttachConsole.Call(uintptr(attachParentProcess))
	if r0 == 0 {
		_, _, _ = pAllocConsole.Call()
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
}

func main() {
	flag.BoolVar(&consoleEnabled, "console", false, "Attach a debug console to the program.")
	flag.Parse()

	onExit := func() {}
	systray.Run(onReady, onExit)

	pFreeConsole.Call()
}

func onReady() {
	systray.SetIcon(notConnected)
	systray.SetTitle(appName)

	menuItemAppName := systray.AddMenuItem(fmt.Sprintf("%s %s", appName, version), "")
	menuItemAppName.Disable()

	systray.AddSeparator()

	menuItemDebugConsole := systray.AddMenuItem("", "")
	toggleConsole(menuItemDebugConsole)
	go func() {
		for {
			<-menuItemDebugConsole.ClickedCh
			consoleEnabled = !consoleEnabled
			toggleConsole(menuItemDebugConsole)
		}
	}()

	menuItemExit := systray.AddMenuItem("Exit", "")
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

	log.SetFlags(log.LstdFlags)
	fmt.Fprintln(os.Stdout, "")

	proc()
}

func proc() {
	for {
		time.Sleep(1 * time.Second)

		log.Println("Opening controller device...")

		d, err := hid.OpenFirst(vendorID, productId)
		if err != nil {
			setStatus(fmt.Sprintf("OpenFirst error: %v", err.Error()))
			systray.SetIcon(notConnected)
			continue
		}

		log.Println("Reading device info...")

		time.Sleep(1 * time.Second)

		info, err := d.GetDeviceInfo()
		if err != nil {
			setStatus(fmt.Sprintf("GetDeviceInfo error: %v", err.Error()))
			systray.SetIcon(notConnected)
			_ = d.Close()
			continue
		}

		var offset int
		var sz int
		switch info.BusType {
		case hid.BusUSB:
			offset = offsetBatteryUSB
			sz = 64
		case hid.BusBluetooth:
			offset = offsetBatteryBT
			sz = 78
		default:
			setStatus(fmt.Sprintf("error: unsupported BusType: %s", info.BusType))
			systray.SetIcon(notConnected)
			_ = d.Close()
			continue
		}

		err = d.SetNonblock(true)
		if err != nil {
			setStatus(fmt.Sprintf("SetNonblock error: %v", err.Error()))
			_ = d.Close()
			continue
		}

		b := make([]byte, sz)

	read:
		log.Printf("Reading status (%d bytes)...", sz)

		_, err = d.Read(b)
		if err != nil {
			setStatus(fmt.Sprintf("Read error: %v", err.Error()))
			systray.SetIcon(notConnected)
			_ = d.Close()
			continue
		}

		// Offsets and calculation referenced from:
		// https://controllers.fandom.com/wiki/Sony_DualSense
		battery0 := b[offset] & battery0Mask
		log.Printf("Read battery0 value of: %x", battery0)

		percent, iconIndex := battery0ToPercentAndIndex(battery0)
		setStatus(fmt.Sprintf("Connection: %s, Battery: %d%%", info.BusType, percent))

		if info.BusType == hid.BusUSB {
			systray.SetIcon(charging[iconIndex])
		} else {
			systray.SetIcon(notCharging[iconIndex])
		}

		// If battery0 is zero and connection is over bluetooth, naively assume this might
		// be a powered down, partial report. Thus send the wake-up byte for 'Get Calibration'.
		// https://controllers.fandom.com/wiki/Sony_DualSense
		if info.BusType == hid.BusBluetooth && battery0 == 0 {
			log.Printf("Writing Bluetooth wake-up byte %x...", wakeupByte)
			for i := range b {
				b[i] = 0
			}
			b[0] = wakeupByte
			_, err = d.SendFeatureReport(b)
			if err != nil {
				setStatus(fmt.Sprintf("Write error: %v", err.Error()))
				_ = d.Close()
				continue
			}
			time.Sleep(1 * time.Second)
			goto read
		}

		_ = d.Close()
	}
}

func setStatus(status string) {
	log.Println(status)
	systray.SetTooltip(status)
}

func toggleConsole(mi *systray.MenuItem) {
	const (
		consoleMenuItemTextShow = "Show debug console"
		consoleMenuItemTextHide = "Hide debug console"
	)
	if consoleEnabled {
		attachConsole()
		log.SetOutput(os.Stdout)
		mi.SetTitle(consoleMenuItemTextHide)
	} else {
		pFreeConsole.Call()
		mi.SetTitle(consoleMenuItemTextShow)
	}
}

// battery0 has the range of 0 - 0x0A, so 10 values.
// Map that to percentage and icon index.
// The 101.0 trick for nIcons avoids branching and fits the
// 100 percents in iconIndex equal to len(icons)-1.
func battery0ToPercentAndIndex(battery0 byte) (int, int) {
	percent := float64(battery0) * 10.0
	nIcons := 101.0 / float64(len(charging))
	iconIndex := math.Floor(percent / nIcons)
	return int(percent), int(iconIndex)
}

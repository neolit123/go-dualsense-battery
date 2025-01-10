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
	vendorID  = 0x054C
	productId = 0x0CE6

	bufferSizeBT  = 78
	bufferSizeUSB = 64

	offsetPowerUSB = 53
	offsetPowerBT  = 54
	maskPowerLevel = 0x0F
	maskPowerState = 0xF0
	maxPowerLevel  = 0x0A

	btReportTruncated = 0x01
	calibrationFR     = 0x05

	stateCharging            = 0x00
	stateDischarging         = 0x01
	stateComplete            = 0x02
	stateAbnormalVoltage     = 0x0A
	stateAbnormalTemperature = 0x0B
	stateChargingError       = 0x0F

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
	//go:embed assets/charging_5.ico
	charging5 []byte
	//go:embed assets/charging_6.ico
	charging6 []byte

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
	//go:embed assets/not_charging_5.ico
	notCharging5 []byte
	//go:embed assets/not_charging_6.ico
	notCharging6 []byte

	charging [][]byte = [][]byte{
		charging0,
		charging1,
		charging2,
		charging3,
		charging4,
		charging5,
		charging6,
	}

	notCharging [][]byte = [][]byte{
		notCharging0,
		notCharging1,
		notCharging2,
		notCharging3,
		notCharging4,
		notCharging5,
		notCharging6,
	}

	k32            = windows.NewLazyDLL("kernel32.dll")
	pAttachConsole = k32.NewProc("AttachConsole")
	pAllocConsole  = k32.NewProc("AllocConsole")
	pFreeConsole   = k32.NewProc("FreeConsole")

	consoleEnabled bool
)

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

func proc() {
	for {
		time.Sleep(1 * time.Second)

		log.Println("Opening controller device...")

		// Randomly throws exception: https://github.com/golang/go/issues/13672
		// Likely doesn't matter if a different cgo wrapper is used, such as:
		// https://github.com/deadsy/hidapi/blob/master/hidapi.go
		// One possible solution might be to have all HID calls in the main thread.
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
			offset = offsetPowerUSB
			sz = bufferSizeUSB
		case hid.BusBluetooth:
			offset = offsetPowerBT
			sz = bufferSizeBT
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

		log.Printf("Reading input report (%d bytes)...", sz)

		sz, err = d.Read(b)
		if err != nil {
			setStatus(fmt.Sprintf("Read error: %v", err.Error()))
			systray.SetIcon(notConnected)
			_ = d.Close()
			continue
		}
		if sz == 0 {
			setStatus("Error: Received a buffer of size 0")
			systray.SetIcon(notConnected)
			_ = d.Close()
			continue
		}

		log.Printf("Received an input report with first byte: %#x", b[0])

		// Sending a calibration report (using calibrationFR) is required to switch BT input
		// reports from the truncated report to the expanded report.
		// https://controllers.fandom.com/wiki/Sony_DualSense
		if info.BusType == hid.BusBluetooth && b[0] == btReportTruncated {
			log.Print("Requesting calibration to wake up the BT device...")

			for i := range b {
				b[i] = 0
			}
			b[0] = calibrationFR

			_, err = d.GetFeatureReport(b)
			if err != nil {
				setStatus(fmt.Sprintf("Write error: %v", err.Error()))
				_ = d.Close()
				continue
			}

			time.Sleep(1 * time.Second)
			continue
		}

		// Offsets and calculation referenced from:
		// https://controllers.fandom.com/wiki/Sony_DualSense
		powerLevel := b[offset] & maskPowerLevel
		powerState := (b[offset] & maskPowerState) >> 4
		log.Printf("Read powerLevel: %#x, powerState: %#x", powerLevel, powerState)
		if powerState == stateComplete {
			powerLevel = maxPowerLevel
			log.Printf("powerLevel adjusted to %#x", powerLevel)
		}

		percent, iconIndex := powerLevelToPercentAndIndex(powerLevel)
		state := parsePowerState(powerState)
		setStatus(fmt.Sprintf("Connection: %s, Power: %d%%, State: %s",
			info.BusType, percent, state))

		if info.BusType == hid.BusUSB {
			systray.SetIcon(charging[iconIndex])
		} else {
			systray.SetIcon(notCharging[iconIndex])
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
		consoleMenuItemTextShow = "Attach debug console"
		consoleMenuItemTextHide = "Detach debug console"
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

// https://controllers.fandom.com/wiki/Sony_DualSense
// Power levels have the range of 0 - 0x0A, so 11 values.
// Map that to percentage and icon index.
// The 101.0 trick for nIcons avoids branching and fits the
// 100 percents in iconIndex equal to len(icons)-1.
func powerLevelToPercentAndIndex(p byte) (int, int) {
	percent := float64(p) * (100.0 / maxPowerLevel)
	nIcons := 101.0 / float64(len(charging))
	iconIndex := math.Floor(percent / nIcons)
	return int(percent), int(iconIndex)
}

// https://controllers.fandom.com/wiki/Sony_DualSense
// 4 bits for state.
func parsePowerState(s byte) string {
	switch s {
	case stateDischarging:
		return "Discharging"
	case stateCharging:
		return "Charging"
	case stateComplete:
		return "Complete"
	case stateAbnormalVoltage:
		return "AbnormalVoltage"
	case stateAbnormalTemperature:
		return "AbnormalTemperature"
	case stateChargingError:
		fallthrough
	default:
		return "ChargingError"
	}
}

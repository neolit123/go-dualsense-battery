// SPDX-License-Identifier: Apache-2.0
// Copyright 2025 the go-dualsense-battery contributors

package main

import (
	_ "embed"
	"fmt"
	"math"
	"time"

	"github.com/getlantern/systray"
	"github.com/sstallion/go-hid"
)

const (
	appName = "go-dualsense-battery"

	vendorID  = 0x054C
	productId = 0x0CE6

	offsetBatteryUSB = 53
	offsetBatteryBT  = 54
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
)

func main() {
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
		systray.SetTooltip(fmt.Sprintf("Init error: %v", err.Error()))
		time.Sleep(2 * time.Second)
		systray.Quit()
	}

open:
	time.Sleep(1 * time.Second)

	systray.SetTooltip("Searching for controller...")

	time.Sleep(1 * time.Second)

	d, err := hid.OpenFirst(vendorID, productId)
	if err != nil {
		systray.SetTooltip(fmt.Sprintf("OpenFirst error: %v", err.Error()))
		systray.SetIcon(notConnected)
		goto open
	}

	systray.SetTooltip("Reading device info...")

	time.Sleep(1 * time.Second)

	info, err := d.GetDeviceInfo()
	if err != nil {
		systray.SetTooltip(fmt.Sprintf("GetDeviceInfo error: %v", err.Error()))
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
		systray.SetTooltip(fmt.Sprintf("error: unsupported BusType: %s", info.BusType))
		systray.SetIcon(notConnected)
		_ = d.Close()
		goto open
	}

	err = d.SetNonblock(true)
	if err != nil {
		systray.SetTooltip(fmt.Sprintf("SetNonblock error: %v", err.Error()))
		_ = d.Close()
		goto open
	}

	systray.SetTooltip("Reading status...")

	time.Sleep(1 * time.Second)

	b := make([]byte, 128)
	_, err = d.Read(b)
	if err != nil {
		systray.SetTooltip(fmt.Sprintf("Read error: %v", err.Error()))
		systray.SetIcon(notConnected)
		_ = d.Close()
		goto open
	}

	// Offsets and calculation referenced from:
	// - https://github.com/Ohjurot/DualSense-Windows
	battery0 := b[offset] & 0xf
	percent := (float64(battery0) / 8.0) * 100.0
	iconIndex := int(math.Round(percent/20)) - 1

	systray.SetTooltip(fmt.Sprintf("Connection: %s, Battery: %.0f%%", info.BusType, percent))

	if info.BusType == hid.BusUSB {
		systray.SetIcon(charging[iconIndex])
	} else {
		systray.SetIcon(notCharging[iconIndex])
	}

	time.Sleep(time.Second * 1)
	_ = d.Close()
	goto open
}

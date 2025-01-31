// SPDX-License-Identifier: Apache-2.0
// Copyright 2025 the go-dualsense-battery authors

package main

import "testing"

func TestPowerLevelToPercentAndIndex(t *testing.T) {
	expected := [][]int{
		{0, 0},
		{10, 0},
		{20, 1},
		{30, 2},
		{40, 2},
		{50, 3},
		{60, 4},
		{70, 4},
		{80, 5},
		{90, 6},
		{100, 6},
	}

	maxPowerLevel1 := maxPowerLevel + 1
	if len(expected) != maxPowerLevel1 {
		t.Fatalf("expected must have %d elements", maxPowerLevel1)
	}

	for i := 0; i < maxPowerLevel1; i++ {
		b0 := byte(i)
		p, idx := powerLevelToPercentAndIndex(b0)
		if p != expected[i][0] {
			t.Errorf("%d: expected p: %d, got: %d", i, expected[i][0], p)
		}
		if idx != expected[i][1] {
			t.Errorf("%d: expected idx: %d, got: %d", i, expected[i][1], idx)
		}
	}
}

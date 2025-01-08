// SPDX-License-Identifier: Apache-2.0
// Copyright 2025 the go-dualsense-battery authors

package main

import "testing"

func TestBattery0ToPercentAndIndex(t *testing.T) {
	expected := [][]int{
		{0, 0},
		{10, 0},
		{20, 0},
		{30, 1},
		{40, 1},
		{50, 2},
		{60, 2},
		{70, 3},
		{80, 3},
		{90, 4},
		{100, 4},
	}

	for i := 0; i < battery0Mask+1; i++ {
		b0 := byte(i)
		p, idx := battery0ToPercentAndIndex(b0)
		if p != expected[i][0] {
			t.Errorf("expected p: %d, got: %d", expected[i][0], p)
		}
		if idx != expected[i][1] {
			t.Errorf("expected idx: %d, got: %d", expected[i][1], idx)
		}
	}
}

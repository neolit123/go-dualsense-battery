// SPDX-License-Identifier: Apache-2.0
// Copyright 2025 the go-dualsense-battery authors

package main

import "testing"

func TestBattery0ToPercentAndIndex(t *testing.T) {
	expected := [][]int{
		{0, 0},
		{6, 0},
		{13, 0},
		{20, 0},
		{26, 1},
		{33, 1},
		{40, 1},
		{46, 2},
		{53, 2},
		{60, 2},
		{66, 3},
		{73, 3},
		{80, 3},
		{86, 4},
		{93, 4},
		{100, 4},
	}

	for i := 0; i < battery0Mask+1; i++ {
		b0 := byte(i)
		p, idx := battery0ToPercentAndIndex(b0)
		if p != expected[i][0] {
			t.Errorf("expected p: %d, got: %d", expected[i][0], p)
		}
		if idx != expected[i][1] {
			t.Errorf("expected idx: %d, got: %d", expected[1][0], idx)
		}
	}
}

// SPDX-License-Identifier: MIT
// Copyright (c) 2026 GREEN-API

package mcp

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// buildGridPNG renders a modules×modules boolean grid as a PNG where each
// module is scale×scale pixels, and returns it base64-encoded.
func buildGridPNG(t *testing.T, grid [][]bool, scale int) string {
	t.Helper()
	n := len(grid)
	img := image.NewGray(image.Rect(0, 0, n*scale, n*scale))
	for y := 0; y < n*scale; y++ {
		for x := 0; x < n*scale; x++ {
			c := color.Gray{Y: 255}
			if grid[y/scale][x/scale] {
				c = color.Gray{Y: 0}
			}
			img.SetGray(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestPngToUnicode(t *testing.T) {
	// 25x25 pseudo-QR: alternating modules with single-module runs so
	// module-size detection finds scale exactly.
	const n = 25
	grid := make([][]bool, n)
	for y := range grid {
		grid[y] = make([]bool, n)
		for x := range grid[y] {
			grid[y][x] = (x+y)%2 == 0
		}
	}
	b64 := buildGridPNG(t, grid, 6)

	out, err := pngToUnicode(b64)
	if err != nil {
		t.Fatalf("pngToUnicode: %v", err)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	var body []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			body = append(body, l)
		}
	}
	if want := (n + 1) / 2; len(body) != want { // 2 module rows per line
		t.Errorf("expected %d content lines, got %d", want, len(body))
	}
	// Checkerboard with odd row pairs → every cell pair is (dark,light) or
	// (light,dark), so output must consist of ▀ and ▄ only (plus margins).
	for _, l := range body {
		trimmed := strings.TrimSpace(l)
		if strings.ContainsAny(trimmed, "█") || !strings.ContainsAny(trimmed, "▀▄") {
			t.Errorf("unexpected content line: %q", trimmed)
		}
		if got := len([]rune(trimmed)); got != n {
			t.Errorf("expected %d module columns, got %d in %q", n, got, trimmed)
		}
	}
}

func TestPngToUnicodeTooFewModules(t *testing.T) {
	grid := [][]bool{{true, false}, {false, true}}
	if _, err := pngToUnicode(buildGridPNG(t, grid, 10)); err == nil {
		t.Error("expected error for image with too few modules")
	}
}

func TestPngToUnicodeInvalidInput(t *testing.T) {
	if _, err := pngToUnicode("not-base64!!!"); err == nil {
		t.Error("expected error for invalid base64")
	}
	if _, err := pngToUnicode(base64.StdEncoding.EncodeToString([]byte("not a png"))); err == nil {
		t.Error("expected error for invalid PNG")
	}
}

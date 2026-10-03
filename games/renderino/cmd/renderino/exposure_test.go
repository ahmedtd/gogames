package main

import (
	"math/rand/v2"
	"testing"
)

func TestExposureHistogramBin(t *testing.T) {
	// Every in-range magnitude lands in a bin whose upper edge is above it,
	// and whose previous bin's upper edge is at or below it.
	rng := rand.New(rand.NewPCG(1, 2))
	for range 10000 {
		mag := float32(1.0 / 256 * float64(rng.Float32()) * 65536)
		bin := exposureHistogramBin(mag)
		if bin == exposureHistogramBins-1 {
			continue
		}
		if mag >= exposureHistogramBinUpperEdge(bin) {
			t.Fatalf("mag %v in bin %d, but upper edge is %v", mag, bin, exposureHistogramBinUpperEdge(bin))
		}
		if bin > 0 && mag < exposureHistogramBinUpperEdge(bin-1) {
			t.Fatalf("mag %v in bin %d, but previous upper edge is %v", mag, bin, exposureHistogramBinUpperEdge(bin-1))
		}
	}

	if got := exposureHistogramBin(0); got != 0 {
		t.Errorf("bin(0) = %d, want 0", got)
	}
	if got := exposureHistogramBin(1e9); got != exposureHistogramBins-1 {
		t.Errorf("bin(1e9) = %d, want %d", got, exposureHistogramBins-1)
	}
	// Bins are 1/16th of an octave wide, so 1.0 is the lower edge of its bin.
	if got := exposureHistogramBinUpperEdge(exposureHistogramBin(1)); got != 1+1.0/16 {
		t.Errorf("upper edge of bin(1) = %v, want %v", got, 1+1.0/16)
	}
}

func TestExposureHistogramPercentileIgnoresHotspot(t *testing.T) {
	// 9999 pixels at 0.8, and one very bright pixel.
	var histogram [exposureHistogramBins]int32
	histogram[exposureHistogramBin(0.8)] += 9999
	histogram[exposureHistogramBin(40)]++

	got := exposureHistogramPercentile(&histogram, 10000, 0.999)
	if got <= 0.8 || got > 0.8*(1+1.0/16)+1e-6 {
		t.Errorf("99.9th percentile = %v, want just above 0.8", got)
	}

	got = exposureHistogramPercentile(&histogram, 10000, 1)
	if got <= 40 || got > 40*(1+1.0/16)+1e-6 {
		t.Errorf("100th percentile = %v, want just above 40", got)
	}
}

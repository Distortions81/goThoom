package main

import (
	"math"
	"math/bits"
	"runtime"
	"sync"
)

// ---------------- Tunables ----------------

// Polyphase resolution: 1024 fractional steps
const phases = 1024

// Lanczos "a" parameter
const a = 4

// Taps per phase
const taps = 2 * a

// Low-pass cutoff as a fraction of source Nyquist
// <1.0 trims near-Nyquist to reduce ringing on crusty 8 kHz assets
const cutoff = 0.95

// Precomputed table: [phase][tap] in Q15
var (
	fracBits     int
	lzW          [phases][taps]int16
	int16BufPool = sync.Pool{
		New: func() any {
			buf := make([]int16, 0, sampleRate)
			return &buf
		},
	}
)

func init() {
	fracBits = bits.TrailingZeros(uint(phases))
	if 1<<fracBits != phases {
		panic("phases must be power of two")
	}
	initLanczosTable()
}

// sincπ(x) = sin(πx)/(πx), with sincπ(0)=1
func sincpi(x float64) float64 {
	if x == 0 {
		return 1.0
	}
	px := math.Pi * x
	return math.Sin(px) / px
}

// Lanczos kernel with bandwidth control.
// L(x) = sinc(c*x) * sinc((c/a)*x), |x|<a; else 0
func lanczos(x float64) float64 {
	ax := math.Abs(x)
	if ax >= a {
		return 0
	}
	return sincpi(cutoff*x) * sincpi((cutoff/float64(a))*x)
}

// Precompute weights in Q15; normalize each phase to exact DC=1.0.
func initLanczosTable() {
	for p := 0; p < phases; p++ {
		// Fractional offset in [0,1)
		f := float64(p) / phases
		// Taps around base sample: k = -a+1 .. +a
		sum := 0.0
		wf := [taps]float64{}
		idx := 0
		for k := -a + 1; k <= a; k++ {
			x := float64(k) - f // relative position
			w := lanczos(x)
			wf[idx] = w
			sum += w
			idx++
		}
		// Normalize and quantize to Q15
		inv := 1.0 / sum
		total := 0
		for i := 0; i < taps; i++ {
			val := math.Round(wf[i] * inv * 32767.0)
			if val > 32767 {
				val = 32767
			} else if val < -32768 {
				val = -32768
			}
			lzW[p][i] = int16(val)
			total += int(lzW[p][i])
		}
		// Nudge the central-ish tap (k=0 -> index a-1) to force exact sum=32768
		diff := 32768 - total
		v := int(lzW[p][a-1]) + diff
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		lzW[p][a-1] = int16(v)
	}
}

func getInt16Buf(n int) []int16 {
	bufPtr := int16BufPool.Get().(*[]int16)
	buf := *bufPtr
	if cap(buf) < n {
		buf = make([]int16, n)
	} else {
		buf = buf[:n]
	}
	*bufPtr = buf
	return buf
}

func putInt16Buf(buf []int16) {
	slice := buf[:0]
	int16BufPool.Put(&slice)
}

// ResampleLanczosInt16PadDB resamples mono int16 from srcRate→dstRate using
// Lanczos-4 (band-limited) with a *built-in dB pad* applied BEFORE int16 quantization.
//
// The returned slice is taken from an internal sync.Pool. Callers must treat it as
// temporary and return it with putInt16Buf once finished. Copy the data if it needs
// to be retained beyond the immediate scope.
// padDB: negative for attenuation (e.g. -3, -6). Non-negative is clamped to 0 dB.
func ResampleLanczosInt16PadDB(src []int16, srcRate, dstRate int, padDB float64) []int16 {
	if len(src) == 0 {
		return nil
	}

	if srcRate == dstRate {
		out := make([]int16, len(src))
		copy(out, src)
		if padDB != 0 {
			return PadDB(out, padDB)
		}
		return out
	}

	origLen := len(src)

	// Output length (safe; includes last contributing center sample).
	n := int((int64(origLen-1)*int64(dstRate))/int64(srcRate)) + 1
	if n <= 0 {
		return nil
	}
	dst := getInt16Buf(n)

	// Q32.32 phase step
	step := (uint64(srcRate) << 32) / uint64(dstRate)

	// Compute Q15 scale from dB once (padDB <= 0 expected).
	scale := math.Pow(10, padDB/20) // e.g., -3 dB ≈ 0.7071
	if scale > 1 {
		scale = 1
	}
	if scale <= 0 {
		scale = 1.0 / (1 << 30) // tiny, avoid zero
	}
	scaleQ15 := int64(math.Round(scale * 32768.0))

	// Pre-pad src with a copies of first and last samples.
	padded := make([]int16, origLen+2*a)
	for i := 0; i < a; i++ {
		padded[i] = src[0]
		padded[origLen+a+i] = src[origLen-1]
	}
	copy(padded[a:], src)
	src = padded

	process := func(start, end int, phase uint64) {
		for i := start; i < end; i++ {
			base := int(phase >> 32)
			fracIdx := int((phase >> (32 - fracBits)) & uint64(phases-1))
			wts := lzW[fracIdx]

			acc := int64(0)
			j := 0
			for k := -a + 1; k <= a; k++ {
				s := int32(src[base+k+a])
				acc += int64(wts[j]) * int64(s)
				j++
			}

			y := int32((acc*scaleQ15 + (1 << 29)) >> 30)
			if y > 32767 {
				y = 32767
			} else if y < -32768 {
				y = -32768
			}
			dst[i] = int16(y)
			phase += step
		}
	}

	const threshold = 2048
	if n < threshold {
		process(0, n, 0)
		return dst
	}

	workers := runtime.NumCPU()
	if workers > n {
		workers = n
	}
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		start := w * n / workers
		end := (w + 1) * n / workers
		phase := step * uint64(start)
		wg.Add(1)
		go func(start, end int, phase uint64) {
			defer wg.Done()
			for i := start; i < end; i++ {
				base := int(phase >> 32)
				fracIdx := int((phase >> (32 - fracBits)) & uint64(phases-1))
				wts := lzW[fracIdx]

				acc := int64(0)
				s := src[base+1 : base+1+taps]
				for j, wt := range wts {
					acc += int64(wt) * int64(s[j])
				}

				y := int32((acc*scaleQ15 + (1 << 29)) >> 30)
				if y > 32767 {
					y = 32767
				} else if y < -32768 {
					y = -32768
				}
				dst[i] = int16(y)
				phase += step
			}
		}(start, end, phase)
	}
	wg.Wait()

	return dst
}

func PadDB(samples []int16, padDB float64) []int16 {
	if padDB == 0 {
		// No pad, just return a copy
		out := make([]int16, len(samples))
		copy(out, samples)
		return out
	}

	scale := math.Pow(10, padDB/20.0)
	out := make([]int16, len(samples))

	for i, s := range samples {
		v := float64(s) * scale
		// Clamp to int16 range
		if v > math.MaxInt16 {
			v = math.MaxInt16
		} else if v < math.MinInt16 {
			v = math.MinInt16
		}
		out[i] = int16(v)
	}

	return out
}

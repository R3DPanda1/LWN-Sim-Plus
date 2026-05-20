package codec

import (
	"fmt"
	"sort"
	"testing"
	"time"
)

type mockDevice struct {
	interval time.Duration
}

func (d *mockDevice) GetSendInterval() time.Duration              { return d.interval }
func (d *mockDevice) SetSendInterval(v time.Duration)             { d.interval = v }
func (d *mockDevice) Print(content string, err error, ptype int)  {}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

func TestCodecExecutionTimes(t *testing.T) {
	cases := []struct {
		name   string
		script string
	}{
		{"AM319_uplink", CreateAM319Codec()},
		{"MCFLW13IO_uplink", CreateMCFLW13IOCodec()},
		{"SDM230_uplink", CreateSDM230Codec()},
	}

	const warmup = 50
	const iters = 1000

	exec := NewExecutor(DefaultExecutorConfig())
	dev := &mockDevice{interval: 30 * time.Second}

	for _, c := range cases {
		state := NewState("0011223344556677")

		for i := 0; i < warmup; i++ {
			if _, _, err := exec.ExecuteEncode(c.script, state, dev); err != nil {
				t.Fatalf("%s warmup err: %v", c.name, err)
			}
		}

		samples := make([]float64, 0, iters)
		for i := 0; i < iters; i++ {
			t0 := time.Now()
			if _, _, err := exec.ExecuteEncode(c.script, state, dev); err != nil {
				t.Fatalf("%s iter %d err: %v", c.name, i, err)
			}
			samples = append(samples, float64(time.Since(t0).Microseconds()))
		}

		sort.Float64s(samples)
		min := samples[0]
		max := samples[len(samples)-1]
		median := percentile(samples, 0.50)
		p95 := percentile(samples, 0.95)
		p99 := percentile(samples, 0.99)

		var sum float64
		for _, v := range samples {
			sum += v
		}
		mean := sum / float64(len(samples))

		fmt.Printf("%-20s n=%d  min=%.0fus  mean=%.0fus  p50=%.0fus  p95=%.0fus  p99=%.0fus  max=%.0fus\n",
			c.name, iters, min, mean, median, p95, p99, max)
	}
}

package forwarder

import (
	"testing"

	"github.com/brocaar/lorawan"
)

func TestShardDistribution(t *testing.T) {
	counts := make(map[int]int)
	for i := 0; i < 1000; i++ {
		eui := lorawan.EUI64{byte(i >> 8), byte(i), 0, 0, 0, 0, 0, 0}
		idx := shardIndex(eui, DefaultNumShards)
		counts[idx]++
	}
	for shard, count := range counts {
		if count > 125 {
			t.Errorf("shard %d has %d devices (poor distribution)", shard, count)
		}
	}
}

func TestShardIndexDeterministic(t *testing.T) {
	eui := lorawan.EUI64{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	first := shardIndex(eui, DefaultNumShards)
	for i := 0; i < 100; i++ {
		if got := shardIndex(eui, DefaultNumShards); got != first {
			t.Fatalf("non-deterministic: got %d, want %d", got, first)
		}
	}
}

func TestSetupCreatesShards(t *testing.T) {
	f := Setup()
	if len(f.shards) != DefaultNumShards {
		t.Fatalf("expected %d shards, got %d", DefaultNumShards, len(f.shards))
	}
	for i, s := range f.shards {
		if s == nil {
			t.Fatalf("shard %d is nil", i)
		}
	}
}

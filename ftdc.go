package ftdc

import (
	"io"
	"time"

	"gopkg.in/mgo.v2/bson"
)

// Chunk represents a 'metric chunk' of data in the FTDC
type Chunk struct {
	Metrics []Metric
	NDeltas int
}

// Map converts the chunk to a map representation.
func (c *Chunk) Map() map[string]Metric {
	m := make(map[string]Metric)
	for _, metric := range c.Metrics {
		m[metric.Key] = metric
	}
	return m
}

// Clip trims the chunk to contain as little data as possible while keeping
// data within the given interval. If the chunk is entirely outside of the
// range, it is not modified and the return value is false.
func (c *Chunk) Clip(start, end time.Time) bool {
	st := start.Unix()
	et := end.Unix()
	var si, ei int
	for _, m := range c.Metrics {
		if m.Key != "start" {
			continue
		}
		mst := int64(m.Value) / 1000
		met := (int64(m.Value) + int64(sum(m.Deltas...))) / 1000
		if met < st || mst > et {
			return false // entire chunk outside range
		}
		if mst > st && met < et {
			return true // entire chunk inside range
		}
		t := mst
		for i := 0; i < c.NDeltas; i++ {
			t += int64(m.Deltas[i]) / 1000
			if t < st {
				si++
			}
			if t < et {
				ei++
			} else {
				break
			}
		}
		if ei+1 < c.NDeltas {
			ei++ // inclusive of end time
		} else {
			ei = c.NDeltas - 1
		}
		break
	}
	c.NDeltas = ei - si
	for _, m := range c.Metrics {
		m.Value += sum(m.Deltas[:si]...)
		m.Deltas = m.Deltas[si : ei+1]
	}
	return true
}

// Chunks takes an FTDC diagnostic file in the form of an io.Reader, and
// yields chunks on the given channel. The channel is closed when there are
// no more chunks.
func Chunks(r io.Reader, c chan<- Chunk) error {
	errCh := make(chan error)
	ch := make(chan bson.D)
	go func() {
		errCh <- readDiagnostic(r, ch)
	}()
	go func() {
		errCh <- readChunks(ch, c)
	}()
	for i := 0; i < 2; i++ {
		err := <-errCh
		if err != nil {
			return err
		}
	}
	return nil
}

// Metric represents an item in a chunk.
type Metric struct {
	// Key is the dot-delimited key of the metric. The key is either
	// 'start', 'end', or starts with 'serverStatus.'.
	Key string

	// Value is the value of the metric at the beginning of the sample
	Value int

	// Deltas is the slice of deltas, which accumulate on Value to yield the
	// specific sample's value.
	Deltas []int
}

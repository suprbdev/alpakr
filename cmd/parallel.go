package cmd

import (
	"errors"
	"sync"
	"time"

	"github.com/suprbdev/alpakr/internal/engine"
	"github.com/suprbdev/alpakr/internal/output"
	"github.com/suprbdev/alpakr/internal/source"
)

const (
	// batchSize caps the records handed to a worker at once. Smaller batches
	// are sent whenever the reader is about to wait for more input.
	batchSize = 256
	// idleFlush is how long the writer waits for a result before flushing.
	idleFlush = 20 * time.Millisecond
)

// rawRecord is an undecoded record, decoded by the worker.
type rawRecord struct {
	data []byte
	pos  int
}

type streamBatch struct {
	seq     int
	records []interface{}
}

type streamItem struct {
	result  interface{}
	encoded []byte // result as written by WriteOne, when the writer supports it
	err     error
}

type batchResult struct {
	seq   int
	items []streamItem
}

// streamParallel processes the records of a stream on n workers and writes
// the results in input order. One goroutine reads records into batches, the
// workers transform (and, for line formats, encode) them, and the calling
// goroutine writes. At most 4n batches are in flight. Results, --limit and
// the first reported error (read or transform) match sequential processing.
// A partial batch is dispatched whenever the reader would wait on input, and
// buffered output is flushed once the writer has waited idleFlush, so slow
// or live input still flows promptly.
func streamParallel(eng *engine.Engine, handler string, limit int, w output.Writer, flush func() error,
	live *liveOutput, pending []interface{}, vals source.Values, n int) error {
	window := 4 * n
	slots := make(chan struct{}, window)
	jobs := make(chan streamBatch)
	results := make(chan batchResult, window)
	readErr := make(chan error, 1)
	readEnd := make(chan int, 1)
	stop := make(chan struct{})
	var workers sync.WaitGroup
	defer func() {
		close(stop)
		workers.Wait()
	}()

	enc, _ := w.(output.LineEncoder)
	raw, _ := vals.(source.RawValues)

	go func() {
		defer close(jobs)
		seq := 0
		var batch []interface{}
		dispatch := func() error {
			if len(batch) == 0 {
				return nil
			}
			select {
			case slots <- struct{}{}:
			case <-stop:
				return errStopped
			}
			select {
			case jobs <- streamBatch{seq: seq, records: batch}:
				seq++
				batch = nil
				return nil
			case <-stop:
				return errStopped
			}
		}
		live.beforeInput = dispatch
		add := func(v interface{}) error {
			batch = append(batch, v)
			if len(batch) >= batchSize {
				return dispatch()
			}
			return nil
		}
		for _, v := range pending {
			if add(v) != nil {
				return
			}
		}
		for {
			var v interface{}
			var ok bool
			var err error
			if raw != nil {
				var data []byte
				var pos int
				data, pos, ok, err = raw.NextRaw()
				v = rawRecord{data: data, pos: pos}
			} else {
				v, ok, err = vals.Next()
			}
			if err != nil || !ok {
				if dispatch() != nil {
					return
				}
				readErr <- err
				readEnd <- seq
				return
			}
			if add(v) != nil {
				return
			}
		}
	}()

	for i := 0; i < n; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for {
				select {
				case b, ok := <-jobs:
					if !ok {
						return
					}
					results <- batchResult{seq: b.seq, items: runBatch(eng, handler, enc, raw, b.records)}
				case <-stop:
					return
				}
			}
		}()
	}

	idle := time.NewTimer(idleFlush)
	defer idle.Stop()
	ready := make(map[int][]streamItem)
	next, written, end := 0, 0, -1
	var endErr error
	receive := func(block bool) error {
		var timeout <-chan time.Time
		if !block {
			idle.Reset(idleFlush)
			timeout = idle.C
		}
		select {
		case r := <-results:
			ready[r.seq] = r.items
		case endErr = <-readErr:
			end = <-readEnd
		case <-timeout:
			if err := flush(); err != nil {
				return err
			}
			return errIdle
		}
		if !block && !idle.Stop() {
			select {
			case <-idle.C:
			default:
			}
		}
		return nil
	}

	for end < 0 || next < end {
		items, ok := ready[next]
		if !ok {
			err := receive(false)
			if errors.Is(err, errIdle) {
				err = receive(true)
			}
			if err != nil {
				return err
			}
			continue
		}
		delete(ready, next)
		next++
		<-slots
		for _, it := range items {
			if it.err != nil {
				return it.err
			}
			if it.result == nil {
				continue
			}
			count := 1
			a, isArr := it.result.([]interface{})
			if isArr {
				count = len(a)
			}
			var err error
			switch {
			case isArr && limit > 0 && written+count > limit:
				count = limit - written
				err = w.WriteOne(a[:count])
			case it.encoded != nil:
				err = enc.WriteEncoded(it.encoded)
			default:
				err = w.WriteOne(it.result)
			}
			if err != nil {
				return err
			}
			written += count
			if limit > 0 && written >= limit {
				return nil
			}
		}
	}
	return endErr
}

var (
	errStopped = errors.New("stopped")
	errIdle    = errors.New("idle")
)

// runBatch decodes and transforms records in order, stopping at the first
// error.
func runBatch(eng *engine.Engine, handler string, enc output.LineEncoder, raw source.RawValues, records []interface{}) []streamItem {
	items := make([]streamItem, 0, len(records))
	for _, rec := range records {
		var res interface{}
		var err error
		if r, ok := rec.(rawRecord); ok {
			rec, err = raw.Decode(r.data, r.pos)
		}
		if err == nil {
			res, err = eng.Run(handler, rec)
		}
		it := streamItem{result: res, err: err}
		if err == nil && res != nil && enc != nil {
			it.encoded, it.err = enc.EncodeLine(nil, res)
		}
		items = append(items, it)
		if it.err != nil {
			break
		}
	}
	return items
}

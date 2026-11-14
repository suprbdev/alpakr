package engine

import "sync"

// parallelChunk is the number of records a worker processes per task.
const parallelChunk = 256

type chunkResult struct {
	out []interface{}
	err error
}

// runItemsParallel is runItems spread over workers. Chunks are assembled in
// input order and a failed chunk keeps the records before its failure, so
// results, the limit and the first reported error match sequential
// processing: an error after the limit-th record is never reported.
func (h *compiledHandler) runItemsParallel(items []interface{}, limit, workers int) (interface{}, error) {
	nChunks := (len(items) + parallelChunk - 1) / parallelChunk
	results := make([]chan chunkResult, nChunks)
	for i := range results {
		results[i] = make(chan chunkResult, 1)
	}
	jobs := make(chan int)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				results[i] <- h.runChunk(items[i*parallelChunk : min((i+1)*parallelChunk, len(items))])
			}
		}()
	}
	go func() {
		defer close(jobs)
		for i := 0; i < nChunks; i++ {
			select {
			case jobs <- i:
			case <-stop:
				return
			}
		}
	}()
	defer func() {
		close(stop)
		wg.Wait()
	}()

	n := len(items)
	if limit > 0 && limit < n {
		n = limit
	}
	out := make([]interface{}, 0, n)
	for i := 0; i < nChunks; i++ {
		r := <-results[i]
		out = append(out, r.out...)
		if limit > 0 && len(out) >= limit {
			return out[:limit], nil
		}
		if r.err != nil {
			return out, r.err
		}
	}
	return out, nil
}

func (h *compiledHandler) runChunk(items []interface{}) chunkResult {
	out := make([]interface{}, 0, len(items))
	for _, item := range items {
		v, err := h.runOne(item, 0)
		if err != nil {
			return chunkResult{out: out, err: err}
		}
		if v != nil {
			out = append(out, v)
		}
	}
	return chunkResult{out: out}
}

package main

import (
	"context"
	"errors"
	"sync"
)

type Result[R any] struct {
	Value R
	Err   error
}

var ErrInvalidWorkers = errors.New("workers must be positive")

func ParallelMap[T any, R any](ctx context.Context, workers int, in []T, fn func(context.Context, T) (R, error)) ([]Result[R], error) {
	if workers <= 0 {
		return nil, ErrInvalidWorkers
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if len(in) == 0 {
		return []Result[R]{}, nil
	}

	workerCount := min(workers, len(in))
	jobs := make(chan int)
	results := make([]Result[R], len(in))

	var wg sync.WaitGroup
	wg.Add(workerCount)
	for range workerCount {
		go func() {
			defer wg.Done()
			for index := range jobs {
				if ctx.Err() != nil {
					return
				}

				value, err := fn(ctx, in[index])
				if err != nil {
					results[index].Err = err
					continue
				}

				results[index].Value = value
			}
		}()
	}

	canceled := false
	for index := range in {
		select {
		case <-ctx.Done():
			canceled = true
		case jobs <- index:
		}

		if canceled {
			break
		}
	}

	close(jobs)
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

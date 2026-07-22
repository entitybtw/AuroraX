package responsecache

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

const janitorInterval = time.Hour

type vecJanitor struct {
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func startVecCleanup(store VecStore) *vecJanitor {
	ctx, cancel := context.WithCancel(context.Background())
	j := &vecJanitor{cancel: cancel}
	j.wg.Add(1)
	go func() {
		defer j.wg.Done()
		t := time.NewTicker(janitorInterval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				if err := store.DeleteExpired(ctx); err != nil {
					if ctx.Err() != nil {
						return
					}
					slog.Warn("vecstore: delete expired", "err", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return j
}

func (j *vecJanitor) close() {
	if j == nil {
		return
	}
	j.cancel()
	j.wg.Wait()
}

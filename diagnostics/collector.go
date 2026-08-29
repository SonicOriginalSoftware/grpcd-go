//revive:disable:package-comments
package diagnostics

import (
	"context"
	"sync"

	diagpb "git.sonicoriginal.software/grpc-protos/diagnostics"
)

// collector gathers the results of checks running on separate goroutines.
// Every goroutine writes into the same map, so the mutex guards the write and
// the wait group marks when the map is safe to read.
type collector struct {
	mu       sync.Mutex
	wg       sync.WaitGroup
	services map[string]*diagpb.ServiceDependency
}

func newCollector(size int) *collector {
	return &collector{services: make(map[string]*diagpb.ServiceDependency, size)}
}

// start runs check on its own goroutine, recording the result under name.
func (c *collector) start(ctx context.Context, name string, check Check) {
	c.wg.Add(1)

	go c.collect(ctx, name, check)
}

// collect runs one check and records its result. A check that fails still
// reports what it learned, so the result is kept either way.
func (c *collector) collect(ctx context.Context, name string, check Check) {
	defer c.wg.Done()

	result, _ := check(ctx)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.services[name] = result
}

// wait blocks until every started check has recorded its result.
func (c *collector) wait() map[string]*diagpb.ServiceDependency {
	c.wg.Wait()

	return c.services
}

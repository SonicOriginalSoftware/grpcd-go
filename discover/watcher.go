package discover

import (
	"context"
	"log/slog"
	"sync"

	"google.golang.org/grpc"

	grpcd "github.com/grpcd/protos"
)

// watcher holds a Watch stream naming one address for as long as its context
// lives, reopening the stream whenever it ends, and delivers each address
// grpcd says to move to.
//
// It runs on its own goroutine so the loop never blocks on opening a stream:
// a transport dropping while grpcd is unreachable still reaches the loop.
type watcher struct {
	// moves carries each address grpcd sends. Closed once the watcher stops.
	moves <-chan string

	// opened is closed once the first stream is open, which is what the loop
	// waits for before closing the stream it is replacing.
	opened <-chan struct{}

	// stop ends the watcher.
	stop context.CancelFunc
}

// watch starts a watcher on address.
func (l *loop) watch(ctx context.Context, address string) *watcher {
	ctx, cancel := context.WithCancel(ctx)

	moves := make(chan string)
	opened := make(chan struct{})

	go l.keepWatching(ctx, address, moves, opened)

	return &watcher{moves: moves, opened: opened, stop: cancel}
}

// keepWatching holds one Watch stream after another until ctx ends.
func (l *loop) keepWatching(
	ctx context.Context, address string, moves chan<- string, opened chan struct{},
) {
	defer close(moves)

	var once sync.Once

	log := l.upstream.log.With(slog.String("address", address))

	request := &grpcd.WatchRequest{MethodName: l.upstream.method, Address: address}

	for ctx.Err() == nil {
		stream, err := l.upstream.discovery.service.Watch(ctx, request, grpc.WaitForReady(true))
		if err != nil {
			log.ErrorContext(ctx, "Failed to open watch", slog.Any("error", err))

			continue
		}

		once.Do(func() { close(opened) })

		for {
			response, err := stream.Recv()
			if err != nil {
				log.InfoContext(ctx, "Watch ended", slog.Any("error", err))

				break
			}

			select {
			case moves <- response.GetAddress():
			case <-ctx.Done():
				return
			}
		}
	}
}

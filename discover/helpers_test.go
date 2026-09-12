package discover

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/stats"

	"git.sonicoriginal.software/grpc-testing/mocks/addr"
	grpcd "git.sonicoriginal.software/grpcd-protos"
)

const method = "/pkg.Service/Method"

// discoverStream stands in for one Discover stream. It offers candidates in
// order and then ends with recvErr, and records what the loop sent back.
type discoverStream struct {
	grpc.ClientStream

	candidates []string
	recvErr    error

	// askErr fails the first send, the one naming the method; reportErr fails
	// every dead report after it.
	askErr    error
	reportErr error

	mu     sync.Mutex
	offers int
	sent   []*grpcd.DiscoverRequest
	closed bool
}

func (s *discoverStream) Recv() (*grpcd.DiscoverResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.offers >= len(s.candidates) {
		if s.recvErr != nil {
			return nil, s.recvErr
		}

		return nil, io.EOF
	}

	address := s.candidates[s.offers]
	s.offers++

	return &grpcd.DiscoverResponse{Address: address}, nil
}

func (s *discoverStream) Send(request *grpcd.DiscoverRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if request.GetDeadAddress() == "" && s.askErr != nil {
		return s.askErr
	}

	if request.GetDeadAddress() != "" && s.reportErr != nil {
		return s.reportErr
	}

	s.sent = append(s.sent, request)

	return nil
}

func (s *discoverStream) CloseSend() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true

	return nil
}

// reported answers with the addresses the loop reported dead on this stream.
func (s *discoverStream) reported() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	dead := []string{}

	for _, request := range s.sent {
		if address := request.GetDeadAddress(); address != "" {
			dead = append(dead, address)
		}
	}

	return dead
}

func (s *discoverStream) wasClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.closed
}

// serviceStub hands out the prepared streams in order, repeating the last one,
// and counts how many times it was asked. The embedded interface supplies
// Register, which discovery never calls.
type serviceStub struct {
	grpcd.GRPCDServiceClient

	streams []*discoverStream
	err     error

	// onCall runs on every Discover with the call's ordinal, so a test can end
	// the loop once it has seen what it needs.
	onCall func(n int)

	// blocks makes Discover wait for its context to end and then fail with
	// that, which is how grpcd being unreachable looks through WaitForReady.
	// released is closed once that wait ends.
	blocks   bool
	released chan struct{}

	mu    sync.Mutex
	calls int
}

func (s *serviceStub) Discover(
	ctx context.Context, _ ...grpc.CallOption,
) (grpc.BidiStreamingClient[grpcd.DiscoverRequest, grpcd.DiscoverResponse], error) {
	s.mu.Lock()
	s.calls++
	n := s.calls
	s.mu.Unlock()

	if s.onCall != nil {
		s.onCall(n)
	}

	if s.blocks {
		<-ctx.Done()
		close(s.released)

		return nil, ctx.Err()
	}

	if s.err != nil {
		return nil, s.err
	}

	return s.streams[min(n, len(s.streams))-1], nil
}

func (s *serviceStub) discovers() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.calls
}

// ccStub records the states the loop pushes, in order.
type ccStub struct {
	resolver.ClientConn

	states chan resolver.State
}

func newCCStub() *ccStub {
	return &ccStub{states: make(chan resolver.State, 16)}
}

func (c *ccStub) UpdateState(state resolver.State) error {
	c.states <- state

	return nil
}

// next answers with the next state pushed, failing the test if the loop stops
// first.
func (c *ccStub) next(t *testing.T) resolver.State {
	t.Helper()

	select {
	case state := <-c.states:
		return state
	case <-t.Context().Done():
		t.Fatal("expected a state to be pushed")

		return resolver.State{}
	}
}

// expectAddress fails the test unless the next state pushed holds exactly
// address.
func (c *ccStub) expectAddress(t *testing.T, address string) {
	t.Helper()

	state := c.next(t)

	if len(state.Addresses) != 1 || state.Addresses[0].Addr != address {
		t.Fatalf("pushed %v, want [%s]", state.Addresses, address)
	}
}

// expectNone fails the test unless the next state pushed is empty.
func (c *ccStub) expectNone(t *testing.T) {
	t.Helper()

	if state := c.next(t); len(state.Addresses) != 0 {
		t.Fatalf("pushed %v, want nothing", state.Addresses)
	}
}

// probeStub fails the addresses named in dead and passes every other.
func probeStub(dead ...string) Probe {
	return func(_ context.Context, address string) error {
		for _, d := range dead {
			if d == address {
				return errors.New("unreachable")
			}
		}

		return nil
	}
}

// newUpstream builds an upstream on a discovery running under ctx.
func newUpstream(ctx context.Context, service grpcd.GRPCDServiceClient, probe Probe) *Upstream {
	return New(ctx, slog.New(slog.DiscardHandler), service, probe).Upstream(method)
}

// running starts a loop for u on its own goroutine and answers with a channel
// closed once it returns.
func running(ctx context.Context, u *Upstream, cc resolver.ClientConn) <-chan struct{} {
	done := make(chan struct{})

	l := &loop{upstream: u, cc: cc, cancel: func() {}}

	go func() {
		defer close(done)

		l.run(ctx)
	}()

	return done
}

// drop tells u's sensor the transport to address opened and then closed.
func drop(ctx context.Context, u *Upstream, address string) {
	ctx = u.sensor.TagConn(ctx, &stats.ConnTagInfo{RemoteAddr: addr.New(address)})
	u.sensor.HandleConn(ctx, &stats.ConnBegin{Client: true})
	u.sensor.HandleConn(ctx, &stats.ConnEnd{Client: true})
}

// await blocks until done is closed, failing the test if it never is.
func await(t *testing.T, done <-chan struct{}, message string) {
	t.Helper()

	select {
	case <-done:
	case <-t.Context().Done():
		t.Fatal(message)
	}
}

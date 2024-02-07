package apiclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/nais/api/pkg/protoapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type TestingHelpers struct {
	t     testing.TB
	conns map[*context.CancelFunc]struct{}
	buf   bytes.Buffer

	lock      sync.Mutex
	hasFailed bool
}

func (t *TestingHelpers) Cleanup(f func()) {
	t.t.Cleanup(f)
}

func (t *TestingHelpers) Logf(format string, args ...interface{}) {
	fmt.Fprintf(&t.buf, format, args...)
}

func (t *TestingHelpers) Errorf(format string, args ...interface{}) {
	fmt.Fprintf(&t.buf, format, args...)
}

func (t *TestingHelpers) FailNow() {
	t.lock.Lock()
	defer t.lock.Unlock()
	if t.hasFailed {
		return
	}

	for conn := range t.conns {
		(*conn)()
	}
	fmt.Println(t.buf.String())
	panic("")
}

type MockServers struct {
	AuditLogs           *protoapi.MockAuditLogsServer
	Reconcilers         *protoapi.MockReconcilersServer
	ReconcilerResources *protoapi.MockReconcilerResourcesServer
	Teams               *protoapi.MockTeamsServer
	Users               *protoapi.MockUsersServer
}

func NewMockClient(t testing.TB) (*APIClient, *MockServers) {
	t.Helper()

	s := grpc.NewServer()

	th := &TestingHelpers{
		t:     t,
		conns: map[*context.CancelFunc]struct{}{},
		buf:   bytes.Buffer{},
	}
	mockServers := &MockServers{
		AuditLogs:           protoapi.NewMockAuditLogsServer(th),
		Reconcilers:         protoapi.NewMockReconcilersServer(th),
		ReconcilerResources: protoapi.NewMockReconcilerResourcesServer(th),
		Teams:               protoapi.NewMockTeamsServer(th),
		Users:               protoapi.NewMockUsersServer(th),
	}

	protoapi.RegisterAuditLogsServer(s, mockServers.AuditLogs)
	protoapi.RegisterReconcilersServer(s, mockServers.Reconcilers)
	protoapi.RegisterReconcilerResourcesServer(s, mockServers.ReconcilerResources)
	protoapi.RegisterTeamsServer(s, mockServers.Teams)
	protoapi.RegisterUsersServer(s, mockServers.Users)

	listener := bufconn.Listen(1024 * 1024)
	dialer := func(ctx context.Context, s string) (net.Conn, error) {
		ctx, cancel := context.WithCancel(ctx)
		th.conns[&cancel] = struct{}{}
		return listener.DialContext(ctx)
	}

	client, err := New("bufconn", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer))
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		if err := s.Serve(listener); err != nil {
			func() {
				defer func() {
					_ = recover()
				}()
				if !errors.Is(err, grpc.ErrServerStopped) {
					t.Error(err)
				}
			}()
		}
	}()

	t.Cleanup(func() {
		s.GracefulStop()
	})

	return client, mockServers
}

type mockDialer struct{}

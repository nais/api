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
	testing.TB
	cancels map[*context.CancelFunc]struct{}
	buf     bytes.Buffer

	lock      sync.Mutex
	hasFailed bool
}

func (t *TestingHelpers) printBuffer() {
	if t.buf.Len() > 0 {
		fmt.Println(t.buf.String())
	}

	for conn := range t.cancels {
		(*conn)()
	}
}

func (t *TestingHelpers) FailNow() {
	// 	// t.lock.Lock()
	// 	// defer t.lock.Unlock()
	// 	// if t.hasFailed {
	// 	// 	return
	// 	// }

	// 	// for conn := range t.cancels {
	// 	// 	(*conn)()
	// 	// }
	// 	// fmt.Print("Fail in the following test:", t.t.Name())
	// 	// fmt.Println(t.buf.String())
	// 	// t.buf.Reset()
	t.TB.Fail()
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
		TB:      t,
		cancels: map[*context.CancelFunc]struct{}{},
		buf:     bytes.Buffer{},
	}
	th.Cleanup(th.printBuffer)
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
		th.cancels[&cancel] = struct{}{}
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

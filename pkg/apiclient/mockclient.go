package apiclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/nais/api/pkg/protoapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type TestingHelpers struct {
	testing.TB
	cancels  map[*context.CancelFunc]struct{}
	buf      bytes.Buffer
	shutdown func()
}

func (t *TestingHelpers) printBuffer() {
	if t.buf.Len() > 0 && t.Failed() {
		fmt.Println(t.Name())
		fmt.Println(t.buf.String())
	}

	for conn := range t.cancels {
		(*conn)()
	}
}

func (t *TestingHelpers) Errorf(format string, args ...interface{}) {
	fmt.Fprintf(&t.buf, format+"\n", args...)
	t.TB.Errorf(format, args...)
}

func (t *TestingHelpers) Logf(format string, args ...interface{}) {
	fmt.Fprintf(&t.buf, format+"\n", args...)
	t.TB.Logf(format, args...)
}

func (t *TestingHelpers) Fail() {
	t.TB.Fail()
	t.shutdown()
}

func (t *TestingHelpers) FailNow() {
	if t.buf.Len() > 0 {
		fmt.Print("Fail in the following test:", t.Name())
		fmt.Println(t.buf.String())
	}
	t.buf.Reset()
	t.shutdown()
	t.Fail()
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

	ctx, shutdown := context.WithCancel(context.Background())
	th := &TestingHelpers{
		TB:       t,
		cancels:  map[*context.CancelFunc]struct{}{},
		buf:      bytes.Buffer{},
		shutdown: shutdown,
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
	dialer := func(_ context.Context, s string) (net.Conn, error) {
		return listener.DialContext(ctx)
	}

	client, err := New("bufconn", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer))
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		<-ctx.Done()
		s.Stop()
	}()

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
		s.Stop()
	})

	return client, mockServers
}

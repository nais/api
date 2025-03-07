package apiclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/nais/api/pkg/apiclient/protoapi"
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
	Reconcilers *protoapi.MockReconcilersServer
	Teams       *protoapi.MockTeamsServer
	Users       *protoapi.MockUsersServer
	Deployments *protoapi.MockDeploymentsServer
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
		Reconcilers: protoapi.NewMockReconcilersServer(th),
		Teams:       protoapi.NewMockTeamsServer(th),
		Users:       protoapi.NewMockUsersServer(th),
		Deployments: protoapi.NewMockDeploymentsServer(th),
	}

	protoapi.RegisterReconcilersServer(s, mockServers.Reconcilers)
	protoapi.RegisterTeamsServer(s, mockServers.Teams)
	protoapi.RegisterUsersServer(s, mockServers.Users)
	protoapi.RegisterDeploymentsServer(s, mockServers.Deployments)

	listener := bufconn.Listen(1024 * 1024)
	dialer := func(_ context.Context, s string) (net.Conn, error) {
		return listener.DialContext(ctx)
	}

	client, err := New(
		"passthrough://",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
	)
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

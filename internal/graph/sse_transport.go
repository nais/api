package graph

// This is a copy of the gqlgen SSE transport with a fix to prevent panics when
// the client disconnects while the keepAlive goroutine is still running.
//
// When the HTTP handler (Do) returns, the underlying response writer becomes
// invalid. If the keepAlive goroutine calls Flush() after that, we get a nil
// pointer dereference panic. The fix is to wait for the keepAlive goroutine to
// finish before returning from Do.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

type (
	SSE struct {
		KeepAlivePingInterval time.Duration
	}

	sseConnection struct {
		mu              sync.Mutex
		f               http.Flusher
		keepAliveTicker *time.Ticker
	}
)

var _ graphql.Transport = SSE{}

func (t SSE) Supports(r *http.Request) bool {
	if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return false
	}
	return r.Method == http.MethodPost && mediaType == "application/json"
}

func (t SSE) Do(w http.ResponseWriter, r *http.Request, exec graphql.GraphExecutor) {
	ctx := r.Context()
	flusher, ok := w.(http.Flusher)
	if !ok {
		SendErrorf(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	c := &sseConnection{
		f: flusher,
	}

	defer c.flush()

	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Content-Type", "application/json")

	params := &graphql.RawParams{}
	start := graphql.Now()
	params.Headers = r.Header
	params.ReadTime = graphql.TraceTiming{
		Start: start,
		End:   graphql.Now(),
	}

	bodyString, err := getRequestBody(r)
	if err != nil {
		gqlErr := gqlerror.Errorf("could not get json request body: %+v", err)
		resp := exec.DispatchError(ctx, gqlerror.List{gqlErr})
		log.Printf("could not get json request body: %+v", err.Error())
		writeJson(w, resp)
		return
	}

	bodyReader := io.NopCloser(strings.NewReader(bodyString))
	if err = jsonDecode(bodyReader, &params); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		gqlErr := gqlerror.Errorf(
			"json request body could not be decoded: %+v body:%s",
			err,
			bodyString,
		)
		resp := exec.DispatchError(ctx, gqlerror.List{gqlErr})
		log.Printf("decoding error: %+v body:%s", err.Error(), bodyString)
		writeJson(w, resp)
		return
	}

	rc, opErr := exec.CreateOperationContext(ctx, params)
	ctx = graphql.WithOperationContext(ctx, rc)

	w.Header().Set("Content-Type", "text/event-stream")
	fmt.Fprint(w, ":\n\n")
	c.flush()

	if t.KeepAlivePingInterval > 0 {
		c.mu.Lock()
		c.keepAliveTicker = time.NewTicker(t.KeepAlivePingInterval)
		c.mu.Unlock()

		done := make(chan struct{})
		go func() {
			defer close(done)
			c.keepAlive(ctx, w)
		}()
		defer func() { <-done }()
	}

	if opErr != nil {
		resp := exec.DispatchError(ctx, opErr)
		writeJsonWithSSE(w, resp)
	} else {
		responses, ctx := exec.DispatchOperation(ctx, rc)
		for {
			response := responses(ctx)
			if response == nil {
				break
			}
			writeJsonWithSSE(w, response)
			c.flush()

			c.resetTicker(t.KeepAlivePingInterval)
		}
	}

	fmt.Fprint(w, "event: complete\n\n")
}

func (c *sseConnection) resetTicker(interval time.Duration) {
	if interval != 0 {
		c.mu.Lock()
		c.keepAliveTicker.Reset(interval)
		c.mu.Unlock()
	}
}

func (c *sseConnection) keepAlive(ctx context.Context, w io.Writer) {
	for {
		select {
		case <-ctx.Done():
			c.keepAliveTicker.Stop()
			return
		case <-c.keepAliveTicker.C:
			fmt.Fprintf(w, ": ping\n\n")
			c.flush()
		}
	}
}

func (c *sseConnection) flush() {
	c.mu.Lock()
	c.f.Flush()
	c.mu.Unlock()
}

func writeJsonWithSSE(w io.Writer, response *graphql.Response) {
	b, err := json.Marshal(response)
	if err != nil {
		panic(err)
	}
	fmt.Fprintf(w, "event: next\ndata: %s\n\n", b)
}

func getRequestBody(r *http.Request) (string, error) {
	if r == nil || r.Body == nil {
		return "", nil
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", fmt.Errorf("unable to get Request Body %w", err)
	}
	return string(body), nil
}

// SendError sends a best effort error to a raw response writer. It assumes the client can
// understand the standard json error response.
func SendError(w http.ResponseWriter, code int, errors ...*gqlerror.Error) {
	w.WriteHeader(code)
	b, err := json.Marshal(&graphql.Response{Errors: errors})
	if err != nil {
		panic(err)
	}
	_, _ = w.Write(b)
}

// SendErrorf wraps SendError to add formatted messages.
func SendErrorf(w http.ResponseWriter, code int, format string, args ...any) {
	SendError(w, code, &gqlerror.Error{Message: fmt.Sprintf(format, args...)})
}

func writeJson(w io.Writer, response *graphql.Response) {
	b, err := json.Marshal(response)
	if err != nil {
		panic(fmt.Errorf("unable to marshal %s: %w", string(response.Data), err))
	}
	_, _ = w.Write(b)
}

func jsonDecode(r io.Reader, val any) error {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	return dec.Decode(val)
}

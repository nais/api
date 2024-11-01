package graphv1

// This is a copy of github.com/99designs/gqlgen/graphql/handler/transport.SSE
// but with ping support.

import (
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
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

type SSE struct{}

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
		transport.SendErrorf(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	defer flusher.Flush()

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
	flusher.Flush()

	if opErr != nil {
		resp := exec.DispatchError(ctx, opErr)
		writeJsonWithSSE(w, resp)
	} else {
		lock := &sync.Mutex{}
		lastMessage := time.Now()
		responses, ctx := exec.DispatchOperation(ctx, rc)

		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					lock.Lock()
					if time.Since(lastMessage) > 30*time.Second {
						fmt.Fprint(w, ":\n\n")
						flusher.Flush()
					}
					lock.Unlock()
				case <-ctx.Done():
					return
				}
			}
		}()

		for {
			response := responses(ctx)
			if response == nil {
				break
			}
			lock.Lock()
			lastMessage = time.Now()
			writeJsonWithSSE(w, response)
			flusher.Flush()
			lock.Unlock()
		}
	}

	fmt.Fprint(w, "event: complete\n\n")
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

func writeJson(w io.Writer, response *graphql.Response) {
	b, err := json.Marshal(response)
	if err != nil {
		panic(err)
	}
	w.Write(b)
}

func jsonDecode(r io.Reader, val interface{}) error {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	return dec.Decode(val)
}

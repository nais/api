package hookd

import (
	"net/http"
)

type httpClient struct {
	client *http.Client
	psk    string
}

func (c *httpClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-PSK", c.psk)
	return c.client.Do(req)
}

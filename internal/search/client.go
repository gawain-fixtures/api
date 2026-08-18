// Package search queries the order search index.
package search

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type response struct {
	StatusCode int
	Body       io.ReadCloser
}

type searchRequest struct {
	ctx   context.Context
	index []string
}

type search func(opts ...func(*searchRequest)) (*response, error)

func (search) WithContext(ctx context.Context) func(*searchRequest) {
	return func(r *searchRequest) { r.ctx = ctx }
}

func (search) WithIndex(index ...string) func(*searchRequest) {
	return func(r *searchRequest) { r.index = index }
}

type esClient struct {
	Search search
}

type Client struct {
	es      *esClient
	timeout time.Duration
}

func NewClient(address string, httpClient *http.Client, timeout time.Duration) *Client {
	do := func(opts ...func(*searchRequest)) (*response, error) {
		req := searchRequest{ctx: context.Background()}
		for _, opt := range opts {
			opt(&req)
		}
		url := fmt.Sprintf("%s/%s/_search", address, strings.Join(req.index, ","))
		httpReq, err := http.NewRequestWithContext(req.ctx, http.MethodPost, url, bytes.NewReader([]byte(`{}`)))
		if err != nil {
			return nil, err
		}
		httpRes, err := httpClient.Do(httpReq)
		if err != nil {
			return nil, err
		}
		return &response{StatusCode: httpRes.StatusCode, Body: httpRes.Body}, nil
	}
	return &Client{es: &esClient{Search: do}, timeout: timeout}
}

// Query runs the match-all query against index and returns the raw response body.
func (c *Client) Query(ctx context.Context, index string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	res, err := c.es.Search(c.es.Search.WithContext(ctx), c.es.Search.WithIndex(index))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("search: unexpected status %d", res.StatusCode)
	}
	return io.ReadAll(res.Body)
}

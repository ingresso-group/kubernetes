package redis

import (
	"context"
	"net/http"
	"time"

	redis "github.com/go-redis/redis/v8"
	"github.com/gregjones/httpcache"
	"k8s.io/klog/v2"
)

type cache struct {
	rdb *redis.Client
}

func cacheKey(key string) string {
	return "kubecache:" + key
}

func (c cache) Get(key string) (resp []byte, ok bool) {
	item, err := c.rdb.Get(context.TODO(), cacheKey(key)).Bytes()
	if err != nil {
		return nil, false
	}
	return item, true
}

// Set saves a response to the cache as key.
func (c cache) Set(key string, resp []byte) {
	_ = c.rdb.Set(context.TODO(), cacheKey(key), resp, 7*24*time.Hour).Err()
}

// Delete removes the response with key from the cache.
func (c cache) Delete(key string) {
	_ = c.rdb.Del(context.TODO(), cacheKey(key)).Err()
}

// NewWithClient returns a new Cache with the given redis connection.
func NewWithClient(client *redis.Client) httpcache.Cache {
	return cache{client}
}

type cacheRoundTripper struct {
	rt *httpcache.Transport
}

//
// TODO: write tests
//

// newCacheRoundTripper creates a roundtripper that reads the ETag on
// response headers and send the If-None-Match header on subsequent
// corresponding requests.
func newCacheRoundTripper(rdb *redis.Client, rt http.RoundTripper) http.RoundTripper {

	t := httpcache.NewTransport(NewWithClient(rdb))
	t.Transport = rt

	return &cacheRoundTripper{rt: t}
}

func (rt *cacheRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return rt.rt.RoundTrip(req)
}

func (rt *cacheRoundTripper) CancelRequest(req *http.Request) {
	type canceler interface {
		CancelRequest(*http.Request)
	}
	if cr, ok := rt.rt.Transport.(canceler); ok {
		cr.CancelRequest(req)
	} else {
		klog.Errorf("CancelRequest not implemented by %T", rt.rt.Transport)
	}
}

func (rt *cacheRoundTripper) WrappedRoundTripper() http.RoundTripper {
	return rt.rt.Transport
}

package redis

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	redis "github.com/go-redis/redis/v8"

	openapi_v2 "github.com/googleapis/gnostic/openapiv2"
	"k8s.io/klog/v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
)

// CachedDiscoveryClient implements the functions that discovery server-supported API groups,
// versions and resources.
type CachedDiscoveryClient struct {
	delegate discovery.DiscoveryInterface

	// prefix is the key prefix.  It must be unique per host:port combination to work well.
	prefix string

	// our redis client
	rdb *redis.Client

	// ttl is how long the cache should be considered valid
	ttl time.Duration

	// mutex protects the variables below
	mutex sync.Mutex

	// ourFiles are all filenames of cache files created by this process
	ourFiles map[string]struct{}
	// invalidated is true if all cache files should be ignored that are not ours (e.g. after Invalidate() was called)
	invalidated bool
	// fresh is true if all used cache files were ours
	fresh bool
}

var _ discovery.CachedDiscoveryInterface = &CachedDiscoveryClient{}

// ServerResourcesForGroupVersion returns the supported resources for a group and version.
func (d *CachedDiscoveryClient) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	filename := filepath.Join(d.prefix, groupVersion, "serverresources.json")
	cachedBytes, err := d.getCachedFile(filename)
	// don't fail on errors, we either don't have a file or won't be able to run the cached check. Either way we can fallback.
	if err == nil {
		cachedResources := &metav1.APIResourceList{}
		if err := runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), cachedBytes, cachedResources); err == nil {
			klog.V(10).Infof("returning cached discovery info from %v", filename)
			return cachedResources, nil
		}
	}

	liveResources, err := d.delegate.ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		klog.V(3).Infof("skipped caching discovery info due to %v", err)
		return liveResources, err
	}
	if liveResources == nil || len(liveResources.APIResources) == 0 {
		klog.V(3).Infof("skipped caching discovery info, no resources found")
		return liveResources, err
	}

	if err := d.writeCachedFile(filename, liveResources); err != nil {
		klog.V(1).Infof("failed to write cache to %v due to %v", filename, err)
	}

	return liveResources, nil
}

// ServerResources returns the supported resources for all groups and versions.
// Deprecated: use ServerGroupsAndResources instead.
func (d *CachedDiscoveryClient) ServerResources() ([]*metav1.APIResourceList, error) {
	_, rs, err := discovery.ServerGroupsAndResources(d)
	return rs, err
}

// ServerGroupsAndResources returns the supported groups and resources for all groups and versions.
func (d *CachedDiscoveryClient) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return discovery.ServerGroupsAndResources(d)
}

// ServerGroups returns the supported groups, with information like supported versions and the
// preferred version.
func (d *CachedDiscoveryClient) ServerGroups() (*metav1.APIGroupList, error) {
	filename := filepath.Join(d.prefix, "servergroups.json")
	cachedBytes, err := d.getCachedFile(filename)
	// don't fail on errors, we either don't have a file or won't be able to run the cached check. Either way we can fallback.
	if err == nil {
		cachedGroups := &metav1.APIGroupList{}
		if err := runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), cachedBytes, cachedGroups); err == nil {
			klog.V(10).Infof("returning cached discovery info from %v", filename)
			return cachedGroups, nil
		}
	}

	liveGroups, err := d.delegate.ServerGroups()
	if err != nil {
		klog.V(3).Infof("skipped caching discovery info due to %v", err)
		return liveGroups, err
	}
	if liveGroups == nil || len(liveGroups.Groups) == 0 {
		klog.V(3).Infof("skipped caching discovery info, no groups found")
		return liveGroups, err
	}

	if err := d.writeCachedFile(filename, liveGroups); err != nil {
		klog.V(1).Infof("failed to write cache to %v due to %v", filename, err)
	}

	return liveGroups, nil
}

func (d *CachedDiscoveryClient) getCachedFile(filename string) ([]byte, error) {
	// after invalidation ignore cache files not created by this process
	d.mutex.Lock()
	_, ourFile := d.ourFiles[filename]
	if d.invalidated && !ourFile {
		d.mutex.Unlock()
		return nil, errors.New("cache invalidated")
	}
	d.mutex.Unlock()

	// Not sure we need to care about cache times

	//ttl, err := d.rdb.TTL(context.TODO(), filename).Result()
	//if err != nil {
	//	return nil, err
	//}

	//if time.Now().After(fileInfo.ModTime().Add(d.ttl)) {
	//	return nil, errors.New("cache expired")
	//}

	// the cache is present and its valid.  Try to read and use it.

	cachedBytes, err := d.rdb.Get(context.TODO(), filename).Bytes()
	if err != nil {
		return nil, err
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.fresh = d.fresh && ourFile

	return cachedBytes, nil
}

func (d *CachedDiscoveryClient) writeCachedFile(filename string, obj runtime.Object) error {

	bytes, err := runtime.Encode(scheme.Codecs.LegacyCodec(), obj)
	if err != nil {
		return err
	}

	// atomic write and update of our map
	d.mutex.Lock()
	defer d.mutex.Unlock()

	_, err = d.rdb.Set(context.TODO(), filename, bytes, d.ttl).Result()
	if err != nil && err != redis.Nil {
		return err
	}

	if err == nil {
		d.ourFiles[filename] = struct{}{}
	}
	return err
}

// RESTClient returns a RESTClient that is used to communicate with API server
// by this client implementation.
func (d *CachedDiscoveryClient) RESTClient() restclient.Interface {
	return d.delegate.RESTClient()
}

// ServerPreferredResources returns the supported resources with the version preferred by the
// server.
func (d *CachedDiscoveryClient) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return discovery.ServerPreferredResources(d)
}

// ServerPreferredNamespacedResources returns the supported namespaced resources with the
// version preferred by the server.
func (d *CachedDiscoveryClient) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return discovery.ServerPreferredNamespacedResources(d)
}

// ServerVersion retrieves and parses the server's version (git version).
func (d *CachedDiscoveryClient) ServerVersion() (*version.Info, error) {
	return d.delegate.ServerVersion()
}

// OpenAPISchema retrieves and parses the swagger API schema the server supports.
func (d *CachedDiscoveryClient) OpenAPISchema() (*openapi_v2.Document, error) {
	return d.delegate.OpenAPISchema()
}

// Fresh is supposed to tell the caller whether or not to retry if the cache
// fails to find something (false = retry, true = no need to retry).
func (d *CachedDiscoveryClient) Fresh() bool {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.fresh
}

// Invalidate enforces that no cached data is used in the future that is older than the current time.
func (d *CachedDiscoveryClient) Invalidate() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.ourFiles = map[string]struct{}{}
	d.fresh = true
	d.invalidated = true
}

// NewCachedDiscoveryClientForConfig creates a new DiscoveryClient for the given config, and wraps
// the created client in a CachedDiscoveryClient. The provided configuration is updated with a
// custom transport that understands cache responses.
func NewCachedDiscoveryClientForConfig(
	config *restclient.Config, discoveryCachePrefix, httpCacheDir string,
	ttl time.Duration,
) (*CachedDiscoveryClient, error) {

	// create redis connection
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	_, err := rdb.Ping(context.TODO()).Result()
	if err == nil {
		// update the given restconfig with a custom roundtripper that
		// understands how to handle cache responses.
		config = restclient.CopyConfig(config)
		config.Wrap(func(rt http.RoundTripper) http.RoundTripper {
			return newCacheRoundTripper(rdb, rt)
		})
	} else {
		fmt.Println("Redis Connection Failed:", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	return newCachedDiscoveryClient(discoveryClient, discoveryCachePrefix, rdb, ttl), nil
}

// NewCachedDiscoveryClient creates a new DiscoveryClient.  prefix is the directory where discovery docs are held.  It must be unique per host:port combination to work well.
func newCachedDiscoveryClient(
	delegate discovery.DiscoveryInterface, prefix string,
	rdb *redis.Client, ttl time.Duration,
) *CachedDiscoveryClient {

	return &CachedDiscoveryClient{
		delegate: delegate,
		prefix:   prefix,
		rdb:      rdb,
		ttl:      ttl,
		ourFiles: map[string]struct{}{},
		fresh:    true,
	}
}

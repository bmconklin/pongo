package main

import (
	"log"
	"time"
	"net/http"
)

// rudimentary cache item
// replace in future with something more viable
type Cache struct {
    Data        map[string]*cacheItem
    MaxSize     int
}

type cacheItem struct {
    response    []byte
    expireTime  time.Time
}

var cache *Cache

func NewCache(size int) *Cache {
    return &Cache{
        Data: make(map[string]*cacheItem),
        MaxSize: size,
    }
}

func (c *Cache) Get(key string) (data []byte, status string) {
    empty := make([]byte, 0)

    if ci, ok := c.Data[key]; !ok {
        return empty, "MISS"
    } else if ci.expireTime.Before(time.Now()) {
        return empty, "EXPIRED"
    } else {
        return ci.response, "HIT"
    }
}

func (c *Cache) Set(key string, data []byte, seconds int) {
    c.Data[key] = &cacheItem{
        response:   data,
        expireTime: time.Now().Add(time.Duration(seconds) * time.Second),
    }
    if len(c.Data) > c.MaxSize {
        go c.PurgeExpired()
    }
}

func (c *Cache) PurgeExpired() {
    t := time.Now()
    for k := range c.Data {
        if c.Data[k].expireTime.Before(t) {
            delete(c.Data, k)
        }
    }
}

// verify the request is cacheable in accordance with HTTP spec
// and configurations for the vhost. TODO: add more requirements
func cacheableRequest(req *http.Request) bool {
    if req.Method != "GET" && req.Method != "HEAD" {
        log.Println(req.Method)
        return false
    }

    return true
}

// verify the response is cacheable in accordance with HTTP spec
// and configurations for the vhost. TODO: add more requirements
func cacheableResponse(resp *http.Response) bool {
    if resp.StatusCode >= 400 {
        return false
    }

    return true
}
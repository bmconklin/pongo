package server

import (
	"log"
	"time"
    "strings"
	"net/http"
    "labix.org/v2/mgo"
    "labix.org/v2/mgo/bson"
    "github.com/bmconklin/golang-lru"
)

// rudimentary cache item
// replace in future with something more viable
type Cache struct {
    Hot     *lru.Cache
    Cold    *mgo.Session
}

type cacheItem struct {
    Key         string      `bson:"_id"`
    Response    []byte      `bson:"response"`
    ExpireTime  time.Time   `bson:"expire_time"`
}

var cache *Cache

func (c *Cache) scheduleCleaner(d time.Duration) {
    t := time.Tick(d)
    for {
        <-t
        c.PurgeExpired()
    }
}

func (c *Cache) coldSet(ci interface{}) error {
    return c.Cold.DB("cache").C("cache").Insert(ci)
}

func (c *Cache) coldGet(key string) (ci *cacheItem, found bool) {
    if err := c.Cold.DB("cache").C("cache").FindId(key).One(&ci); err != nil {
        return nil, false
    }
    return ci, true
}

func (c *Cache) coldExpire(ci interface{}) error {
    return c.Cold.DB("cache").C("cache").Remove(ci)
}

func NewCache() *Cache {
    lruCache, err := lru.New(config.Cache["hot"].Size)
    if err != nil {
        log.Panic(err)
    }
    dbCache, err  := mgo.Dial("localhost")
    if err != nil {
        log.Panic(err)
    }
    c := &Cache{
        Hot:  lruCache,
        Cold: dbCache,
    }

    c.Hot.OnRemove(func(i interface{}) {
            ci := i.(cacheItem)
            if ci.ExpireTime.After(time.Now()) {
                c.coldSet(&ci)
            }
        })

    go c.scheduleCleaner(60 * time.Second)
    return c
}

func (c *Cache) Get(key string) (data []byte, status string) {
    empty := make([]byte, 0)

    var ci *cacheItem
    var hot bool
    i, ok := c.Hot.Get(key)
    if !ok {
        ci, ok = c.coldGet(key)
        hot = false
    } else {
        cii := i.(cacheItem)
        ci = &cii
        hot = true
    }
    if !ok {
        return empty, "MISS"
    } else if ci.ExpireTime.Before(time.Now()) {
        if !hot {
            c.Cold.DB("cache").C("cache").Remove(ci)
        }
        return ci.Response, "EXPIRED"
    } else {
        if !hot {
            c.Set(ci.Key, ci.Response, ci.ExpireTime)
        }
        return ci.Response, "HIT"
    }
}

func (c *Cache) Set(key string, data []byte, t time.Time) {
    c.Hot.Add(key, cacheItem{
        Key:        key,
        Response:   data,
        ExpireTime: t,
    })
    if c.Hot.Len() > config.Cache["hot"].Size {
        go c.PurgeExpired()
    }
}

func (lc *LocationConfig) GetCacheKey(r *http.Request) string {
    replacer := strings.NewReplacer(
        "$scheme", r.URL.Scheme, 
        "$host", r.Host, 
        "$uri", r.URL.Path, 
        "$querystring", r.URL.RawQuery,
        "$method", r.Method,
    )
    return replacer.Replace(lc.CacheKey)
}

func (c *Cache) PurgeExpired() {
    t := time.Now()
    c.Cold.DB("cache").C("cache").Remove(bson.M{"expire_time": bson.M{"$lte": t}})
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
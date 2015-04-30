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
    Cold    *mgo.GridFS
}

type cacheItem struct {
    Key         string      `bson:"_id"`
    Response    []byte      `bson:"response"`
    ExpireTime  time.Time   `bson:"expire_time"`
}

var cache *Cache

func (c *Cache) coldSet(ci *cacheItem) error {
    gf, err := c.Cold.Create(ci.Key)
    if err != nil {
        return err
    }
    gf.SetId(ci.Key)
    gf.SetMeta(bson.M{"expiretime": ci.ExpireTime})
    gf.Write(ci.Response)
    return gf.Close()
}

func (c *Cache) coldGet(key string) (ci *cacheItem, found bool) {
    gf, err := c.Cold.OpenId(key)
    if err != nil {
        return nil, false
    }
    data := make([]byte, gf.Size())
    if _, err := gf.Read(data); err != nil {
        log.Println(err)
        return nil, false
    }
    result := &struct {
        Expiretime time.Time 
    }{}
    if err := gf.GetMeta(result); err != nil {
        return nil, false
    }
    ci = &cacheItem{
        Key: gf.Id().(string),
        Response: data,
        ExpireTime: result.Expiretime,
    }
    return ci, true
}

func (c *Cache) coldExpire(ci *cacheItem) error {
    return c.Cold.Remove(ci.Key)
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
        Cold: dbCache.DB("cache").GridFS("cache"),
    }

    c.Hot.OnRemove(func(i interface{}) {
            ci := i.(cacheItem)
            if ci.ExpireTime.After(time.Now()) {
                c.coldSet(&ci)
            }
        })

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
            c.coldExpire(ci)
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
    }, int64(len(data)))
}

func (lc *LocationConfig) GetCacheKey(origin string, r *http.Request) string {
    replacer := strings.NewReplacer(
        "$scheme", r.URL.Scheme, 
        "$host", origin, 
        "$uri", r.URL.Path, 
        "$querystring", r.URL.RawQuery,
        "$method", r.Method,
    )
    return replacer.Replace(lc.CacheKey)
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
package main

import (
	"log"
	"time"
	"net/http"
)

// rudimentary cache item
// replace in future with something more viable
type cache struct {
    response    []byte
    expireTime  time.Time
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
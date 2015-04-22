package main

import (
	"log"
	"time"
	"net/url"
	"net/http"
)

// Holds Log data for each request
type Logger struct {
    Method          string
    URL             *url.URL
    Host            string
    RemoteAddr      string
    Status          string
    StatusCode      int
    Proto           string
    ContentLength   int64
    RequestTime     time.Duration
    OriginTime      time.Duration
    Timestamp       time.Time
    CacheStatus     string
}

var logger []LogConfig

// Copy values from the request to the log 
func (l *Logger) ParseReq(req *http.Request) {
    l.Method        = req.Method
    l.URL           = req.URL
    l.Host          = req.Host
    l.RemoteAddr    = req.RemoteAddr
    l.Proto         = req.Proto
    l.Timestamp     = time.Now()
}

// Copy values from the response to the log
func (l *Logger) ParseResp(resp *http.Response) {
    l.Status        = resp.Status
    l.StatusCode    = resp.StatusCode
    l.ContentLength = resp.ContentLength
    l.RequestTime   = time.Since(l.Timestamp)
}

// TODO: Pring log using file path and format from config file
func (l *Logger) Log() {
    log.Println(l)
}
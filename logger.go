package main

import (
    "os"
	"log"
	"time"
    "bufio"
    "strconv"
    "strings"
	"net/url"
	"net/http"
)

// Holds Log data for each request
type AccessLog struct {
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
    Referer         string
    UserAgent       string
}

type AccessLogger struct {
    Logger *log.Logger
    Format string
}

var logger []LogConfig
var accessLogger []*AccessLogger

// Copy values from the request to the log 
func (l *AccessLog) ParseReq(req *http.Request) {
    l.Method        = req.Method
    l.URL           = req.URL
    l.Host          = req.Host
    l.RemoteAddr    = req.RemoteAddr
    l.Proto         = req.Proto
    l.Timestamp     = time.Now()
    l.Referer       = req.Referer()
    l.UserAgent     = req.UserAgent()
}

// Copy values from the response to the log
func (l *AccessLog) ParseResp(resp *http.Response) {
    l.Status        = resp.Status
    l.StatusCode    = resp.StatusCode
    l.ContentLength = resp.ContentLength
    l.RequestTime   = time.Since(l.Timestamp)
}

func getAccessLogs() {
    accessLogger = make([]*AccessLogger, 0)
    for i := range logger {
        if logger[i].Type == "access" {
            file, err := os.Open(logger[i].Location)
            if err != nil {
                file, err = os.Create(logger[i].Location)
                if err != nil {
                    panic(err)
                }
            }
            buf := bufio.NewWriter(file)
            accessLogger = append(accessLogger, &AccessLogger{
                    Logger: log.New(buf, "Pongo access:", 0),
                    Format: logger[i].Format,
                })
        }
    }
}

// TODO: Print log using file path and format from config file
func (l *AccessLog) Log() {
    if len(accessLogger) == 0 {
        getAccessLogs()
    }
    hostname, _ := os.Hostname()
    accessLogReplacer := strings.NewReplacer(
            "$body_bytes_sent", strconv.FormatInt(l.ContentLength, 10),
            "$remote_addr", l.RemoteAddr,
            "$hostname", hostname,
            "$cache_status", l.CacheStatus,
            "$http_host", l.URL.Host,
            "$request_method", l.Method,
            "$origin_response_time", l.OriginTime.String(),
            "$server_protocol", l.Proto,
            "$zone_query_string", l.URL.RawQuery,
            "$http_referer", l.Referer,
            "$scheme", l.URL.Scheme,
            "$zone_status", "",
            "$msec", l.Timestamp.Format("2006-01-02_15:04:05.000"),
            "$uri", l.URL.Path,
            "$http_user_agent", l.UserAgent,
            "$request_time", l.RequestTime.String(),
        )
    for i := range accessLogger {
        accessLogger[i].Logger.Println(accessLogReplacer.Replace(accessLogger[i].Format))
    }
}
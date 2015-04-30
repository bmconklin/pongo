package server

import (
    "os"
    "log"
    "time"
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
    Scheme          string
    Size            int64
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

var accessLogger []*AccessLogger

func NewAccessLog() *AccessLog{
    hostname, _ := os.Hostname()

    l := new(AccessLog)
    l.Timestamp = time.Now()
    l.Host      = hostname
    return l
}

// Copy values from the request to the log 
func (l *AccessLog) ParseReq(req *http.Request) {
    l.Method        = req.Method
    l.URL           = req.URL
    l.Host          = req.Host
    l.RemoteAddr    = req.RemoteAddr
    l.Proto         = req.Proto
    l.Referer       = req.Referer()
    l.UserAgent     = req.UserAgent()
}

// Copy values from the response to the log
func (l *AccessLog) ParseResp(resp *http.Response) {
    l.Status        = resp.Status
    l.StatusCode    = resp.StatusCode
    l.RequestTime   = time.Since(l.Timestamp)
    l.URL           = resp.Request.URL
}

func openAccessLogs() {
    accessLogger = make([]*AccessLogger, 0)
    for _, l := range config.Logs {
        if l.Type == "access" {
            file, err := os.OpenFile(l.Location, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
            if err != nil {
                panic(err)
            }
            accessLogger = append(accessLogger, &AccessLogger{
                log.New(file, "Pongo: ", 0),
                l.Format,
            })
        }
    }
}

func (l *AccessLog) Log() {
    if len(accessLogger) == 0 {
        openAccessLogs()
    }
    accessLogReplacer := strings.NewReplacer(
            "$body_bytes_sent", strconv.FormatInt(l.Size, 10),
            "$remote_addr", l.RemoteAddr,
            "$hostname", l.Host,
            "$cache_status", l.CacheStatus,
            "$http_host", l.Host,
            "$request_method", l.Method,
            "$origin_response_time", l.OriginTime.String(),
            "$server_protocol", l.Proto,
            "$zone_query_string", l.URL.RawQuery,
            "$http_referer", l.Referer,
            "$scheme", l.Scheme,
            "$status", strconv.Itoa(l.StatusCode),
            "$msec", l.Timestamp.Format("2006-01-02_15:04:05.000"),
            "$uri", l.URL.Path,
            "$http_user_agent", l.UserAgent,
            "$request_time", l.RequestTime.String(),
        )

    for i := range accessLogger {
        accessLogger[i].Logger.Println(accessLogReplacer.Replace(accessLogger[i].Format))
    }
}
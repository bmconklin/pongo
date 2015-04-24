package main

import(
    "io"
    "log"
    "net"
    "sync"
    "flag"
    "time"
    "bufio"
    "bytes"
    "net/url"
    "strings"
    "runtime"
    "net/http"
    "net/http/httputil"
)

// Wrapper for http.Handler interface, used to implement serveHTTP()
type proxyHandler struct {
    http.Handler
}

type Target struct {
    Active  bool
    Waiting int
    Chan    chan []byte
}

type ActiveRequests struct{
    Lock    sync.Mutex
    Targets map[string]*Target
}

// any flags passed in at runtime
var (
    configDir   = flag.String("conf", "/etc/pongo/conf/pongo.conf", "location of config file")
    vhostDir    = flag.String("dir", "/etc/pongo/conf/vhosts", "root directory for vhost configs")
)

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var hopHeaders = []string{
    "Connection",
    "Keep-Alive",
    "Proxy-Authenticate",
    "Proxy-Authorization",
    "Te",   // canonicalized version of "TE"
    "Trailers",
    "Transfer-Encoding",
    "Upgrade",
}

func (ar *ActiveRequests) Start(target string) bool {
    ar.Lock.Lock()
    defer ar.Lock.Unlock()
    if _, ok := ar.Targets[target]; !ok {
        ar.Targets[target] = &Target{
            Chan: make(chan []byte),
        }
    }
    if !ar.Targets[target].Active {
        ar.Targets[target].Active = true
        return true
    }
    ar.Targets[target].Waiting++
    return false
}

func (ar *ActiveRequests) Stop(target string, b []byte) {
    ar.Lock.Lock()
    defer ar.Lock.Unlock()
    ar.Targets[target].Active = false
    for i := 0; i < ar.Targets[target].Waiting; i++ {
        ar.Targets[target].Chan <- b
    }
}

func (ar *ActiveRequests) Wait(target string) []byte {
    b := <- ar.Targets[target].Chan
    return b
}

func copyHeader(dst, src http.Header) {
    for k, vv := range src {
        for _, v := range vv {
            dst.Add(k, v)
        }
    }
}

// send response to the client
func respond(res *http.Response, rw http.ResponseWriter) {
    copyHeader(rw.Header(), res.Header)
    rw.WriteHeader(res.StatusCode)
    io.Copy(rw, res.Body)
}

func headerControl(v *vHost, resp *http.Response) {
    for k, v := range config.SetHeader {
        resp.Header.Add(k,v)
    }
    for k, v := range v.SetHeader {
        resp.Header.Add(k, v)
    }
}

func proxy(v *vHost, req  *http.Request) (*http.Response, error) {
    transport := v.Proxy.Transport
    if transport == nil {
        transport = http.DefaultTransport
    }

    outreq := new(http.Request)
    *outreq = *req // includes shallow copies of maps, but okay
    v.Proxy.Director(outreq)    
    outreq.Proto = "HTTP/1.1"
    outreq.ProtoMajor = 1
    outreq.ProtoMinor = 1
    outreq.Close = false

    // Remove hop-by-hop headers to the backend.  Especially
    // important is "Connection" because we want a persistent
    // connection, regardless of what the client sent to us.  This
    // is modifying the same underlying map from req (shallow
    // copied above) so we only copy it if necessary.
    copiedHeaders := false
    for _, h := range hopHeaders {
        if outreq.Header.Get(h) != "" {
            if !copiedHeaders {
                outreq.Header = make(http.Header)
                copyHeader(outreq.Header, req.Header)
                copiedHeaders = true
            }
            outreq.Header.Del(h)
        }
    }

    if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
        // If we aren't the first proxy retain prior
        // X-Forwarded-For information as a comma+space
        // separated list and fold multiple headers into one.
        if prior, ok := outreq.Header["X-Forwarded-For"]; ok {
            clientIP = strings.Join(prior, ", ") + ", " + clientIP
        }
        outreq.Header.Set("X-Forwarded-For", clientIP)
    }

    resp, err := transport.RoundTrip(outreq)
    if err != nil {
        log.Println("http: proxy error: %v", err)
        return resp, err
    }

    for _, h := range hopHeaders {
        resp.Header.Del(h)
    }

    headerControl(v, resp)

    return resp, err
}

// handler method
// Gets config from vHost (should already be in memory) hashmap
// If request is cached, serve from cache
// Otherwise proxy and cache the response according to config
func (p proxyHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
    l := new(AccessLog)
    l.ParseReq(req)
    var b []byte
    if _, ok := vHosts[req.Host]; !ok {
        log.Println("Couldn't find Vhost:", req.Host)
        rw.WriteHeader(http.StatusInternalServerError)
        return
    }
    origin, _ := url.Parse(vHosts[req.Host].Origin)
    cacheKey := vHosts[req.Host].GetCacheKey(req)
    data, status := cache.Get(cacheKey)
    if status == "MISS" || status == "EXPIRED" {
        // ORDER OF CONDITIONS IS VERY IMPORTANT
        if !cacheableRequest(req) || vHosts[req.Host].ByPass || vHosts[req.Host].ActiveRequests.Start(cacheKey)  {
            var err error

            t := time.Now()

            resp, err := proxy(vHosts[req.Host], req)
            if err != nil {
                log.Println("http: proxy error:", err)
                rw.WriteHeader(http.StatusInternalServerError)
                vHosts[req.Host].ActiveRequests.Stop(cacheKey, b)
                return
            }
            l.OriginTime = time.Since(t)

            b, err = httputil.DumpResponse(resp, true)
            if err != nil {
                if status == "EXPIRED" {
                    status = "STALE"
                    b = data
                } else {
                    log.Println("Error reading proxy response:", err)
                    rw.WriteHeader(http.StatusInternalServerError)
                    vHosts[req.Host].ActiveRequests.Stop(cacheKey, b)
                    return
                }
            }
            if cacheableRequest(req) && cacheableResponse(resp) && !vHosts[req.Host].ByPass {
                cache.Set(cacheKey, b, vHosts[req.Host].Expire)
                vHosts[req.Host].ActiveRequests.Stop(cacheKey, b)
            }
        } else {
            b = vHosts[req.Host].ActiveRequests.Wait(cacheKey)
            status = "COLLAPSED"
        }
    }
    if status == "HIT" {
        b = data
    }    
    l.CacheStatus = status
    l.Scheme = origin.Scheme

    buf := bytes.NewBuffer(b)
    resp, err := http.ReadResponse(bufio.NewReader(buf), req)
    if err != nil {
        log.Println(err)
        rw.WriteHeader(http.StatusInternalServerError)
        return
    }
    respond(resp, rw)
    l.ParseResp(resp)
    l.Log()
}

// initialize global settings
func init() {
    flag.Parse()
    runtime.GOMAXPROCS(runtime.NumCPU())
    if err := loadConfig(*configDir); err != nil {
        log.Panic(err)
    }
    loadConfigs(*vhostDir)
    cache = NewCache(1024)
}

// start server
func main() {
    p := proxyHandler{}
    log.Fatal(http.ListenAndServe("0.0.0.0:80", p))
}

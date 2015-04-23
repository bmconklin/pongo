package main

import(
    "io"
    "log"
    "net"
    "flag"
    "time"
    "bufio"
    "bytes"
    "strings"
    "runtime"
    "net/http"
    "net/http/httputil"
)

// Wrapper for http.Handler interface, used to implement serveHTTP()
type proxyHandler struct {
    http.Handler
}

// any flags passed in at runtime
var (
    configDir   = flag.String("conf", "/etc/pongo/conf/prongo.conf", "location of config file")
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

    res, err := transport.RoundTrip(outreq)
    if err != nil {
        log.Println("http: proxy error: %v", err)
        return res, err
    }

    for _, h := range hopHeaders {
        res.Header.Del(h)
    }

    return res, err
}

// handler method
// Gets config from vHost (should already be in memory) hashmap
// If request is cached, serve from cache
// Otherwise proxy and cache the response according to config
func (p proxyHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
    l := new(Logger)
    l.ParseReq(req)
    url := req.URL.String()

    var b []byte
    if _, ok := vHosts[req.Host]; !ok {
        log.Println("Couldn't find Vhost:", req.Host)
        rw.WriteHeader(http.StatusInternalServerError)
        return
    }
    data, status := cache.Get(req.Host + url)
    if status == "MISS" || status == "EXPIRED" {
        var err error

        t := time.Now()
        resp, err := proxy(vHosts[req.Host], req)
        if err != nil {
            log.Println("http: proxy error: %v", err)
            rw.WriteHeader(http.StatusInternalServerError)
            return
        }
        l.OriginTime = time.Since(t)
        b, err = httputil.DumpResponse(resp, true)
        if err != nil {
            if status == "EXPIRED" {
                status = "STALE"
                b = data
            } else {
                log.Println(err)
                rw.WriteHeader(http.StatusInternalServerError)
                return
            }
        }
        if cacheableRequest(req) && cacheableResponse(resp) {
            cache.Set(req.Host + url, b, vHosts[req.Host].Expire)
        }
    }
    if status == "HIT" {
        b = data
    }    
    l.CacheStatus = status
    buf := bytes.NewBuffer(b)
    resp, err := http.ReadResponse(bufio.NewReader(buf), req)
    if err != nil {
        log.Println(err)
    }
    respond(resp, rw)
    l.ParseResp(resp)
    l.Log()
}

// initialize global settings
func init() {
    flag.Parse()
    runtime.GOMAXPROCS(runtime.NumCPU())
    loadConfig(*configDir)
    loadConfigs(*vhostDir)
    cache = NewCache(1024)
}

// start server
func main() {
    p := proxyHandler{}
    log.Fatal(http.ListenAndServe("0.0.0.0:80", p))
}

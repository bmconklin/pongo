package main

import(
    "os"
    "log"
    "flag"
    "time"
    "errors"
    "net/url"
    "runtime"
    "net/http"
    "io/ioutil"
    "encoding/json"
    "net/http/httputil"
)

type cache struct {
    key         string
    data        []byte
    expireTime  time.Time
}

type vHost struct {
    Origin  string                  `json:"origin"`
    VHosts  []string                `json:"vhosts"`
    Expire  int                     `json:"expire"`
    Cache   map[string]cache
    Conn    *httputil.ReverseProxy
}

type proxyHandler struct {
    http.Handler
}

var vHosts map[string]*vHost

var (
    vhostDir = flag.String("dir", "/etc/pongo/vhosts", "root directory for vhost configs")
)

// Reads a config file and parses them into a vHost struct.
// For each vhost associated with a config file, a hashmap
// will be created with a pointer to the vHost struct.
func getConfig(path string) error {
    file, err := os.Open(path)
    if err != nil {
        return errors.New("Error reading from "+ path +". Error returned: " + err.Error())
    }
    defer file.Close()

    var config vHost
    jsonDecoder := json.NewDecoder(file)
    if err = jsonDecoder.Decode(&config); err != nil {
        return errors.New("Unable to decode config file. " + err.Error())
    }
    
    remote, err := url.Parse(config.Origin)
    if err != nil {
        return err
    }

    config.Cache = make(map[string]cache)
    config.Conn =  httputil.NewSingleHostReverseProxy(remote)
    for _, v := range config.VHosts {
        vHosts[v] = &config
    }

    return nil
}

// Recursively searches through subdirectories of the vhost
// root directory and loads every file found that matches
// the config file format. Will print an error if config is 
// not in the proper format (currently JSON).
func loadConfigs(dir string) {
    if vHosts == nil {
        vHosts = make(map[string]*vHost)
    } 
    files, err := ioutil.ReadDir(dir)
    if err != nil {
        log.Println("Could not read from directory:", dir, "Error:",err)
    }
    for _, f := range files {
        if f.IsDir() {
            loadConfigs(dir+"/"+f.Name())
        } else if err := getConfig(dir+"/"+f.Name()); err != nil {
            log.Println(err)
        }
    }
}

func init() {
    flag.Parse()
    runtime.GOMAXPROCS(runtime.NumCPU())
    loadConfigs(*vhostDir)
}

func (p proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    log.Println("Requested", r.Host + r.URL.String())
    t := time.Now()
    if _, ok := vHosts[r.Host]; !ok {
        log.Println("Couldn't find Vhost:", r.Host)
    } else {
        vHosts[r.Host].Conn.ServeHTTP(w, r)
    }
    log.Println("Completed request in", time.Since(t), "with response", )
}

func main() {
    p := proxyHandler{}
    log.Fatal(http.ListenAndServe("0.0.0.0:80", p))
}

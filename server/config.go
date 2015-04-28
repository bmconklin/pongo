package server

import(
    "os"
    "log"
    "errors"
    "net/url"
    "io/ioutil"
    "encoding/json"
    "net/http/httputil"
)

type LocationConfig struct {
    Origin          string                  `json:"origin"`
    CacheKey        string                  `json:"cache_key"`
    Expire          int                     `json:"expire"`
    SetHeader       map[string]string       `json:"set_header"`
    ByPass          bool                    `json:"cache_bypass"`
    Proxy           *httputil.ReverseProxy  `json:"-"`
    ActiveRequests  *ActiveRequests         `json:"-"`
}

// config for a vhost
type vHost struct {
    Port            int                         `json:"port"`
    VHosts          []string                    `json:"vhosts"`
    Location        map[string]*LocationConfig  `json:"location"`
}

// Configuration settings for a log
type LogConfig struct {
    Type        string      `json:"type"`
    Location    string      `json:"location"`
    Format      string      `json:"format"`
    Verbose     bool        `json:"verbose"`
}

// Global Config structure
type Config struct {
    Server      string        
    Port        int
    Cache   struct{
        Type    string
        Size    int
    }
    Logs        []LogConfig             `json:"logs"`
    SetHeader   map[string]string       `json:"set_header"`
    VhostPath   string                  `json:"vhostpath"`
}

// hashmap of vhost to config
var vHosts map[string]*vHost
var config Config

func (v *vHost) String() string {
    b, err := json.Marshal(v)
    if err != nil {
        log.Println(err)
    }
    return string(b)
}

func (v *vHost) PrettyString() string {
    b, err := json.MarshalIndent(v, "", "  ")
    if err != nil {
        log.Println(err)
    }
    return string(b)
}

// Load global config file
func LoadConfig(path string) error {
    file, err := os.Open(path)
    if err != nil {
        return errors.New("Error reading from "+ path +". Error returned: " + err.Error())
    }
    defer file.Close()

    jsonDecoder := json.NewDecoder(file)
    if err = jsonDecoder.Decode(&config); err != nil {
        return errors.New("Unable to decode config file. " + err.Error())
    }

    log.Println("Loaded Pongo config from", path)
    return nil
}

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
    
    for path, cfg := range config.Location {
        remote, err := url.Parse(cfg.Origin)
        if err != nil {
            return err
        }

        config.Location[path].Proxy = httputil.NewSingleHostReverseProxy(remote)
        config.Location[path].ActiveRequests = &ActiveRequests{
            Targets: make(map[string]*Target),
        }
    }

    for _, v := range config.VHosts {
        vHosts[v] = &config
    }
    log.Println("Loaded config for", path)
    return nil
}

// Recursively searches through subdirectories of the vhost
// root directory and loads every file found that matches
// the config file format. Will print an error if config is 
// not in the proper format (currently JSON).
func loadVhosts(dir string) error {
    if vHosts == nil {
        vHosts = make(map[string]*vHost)
    } 
    files, err := ioutil.ReadDir(dir)
    if err != nil {
        return err
    }
    for _, f := range files {
        if f.IsDir() {
            loadVhosts(dir+"/"+f.Name())
        } else if err := getConfig(dir+"/"+f.Name()); err != nil {
            return err
        }
    }
    return nil
}
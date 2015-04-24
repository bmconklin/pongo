package main

import(
    "os"
    "log"
    "errors"
    "net/url"
    "io/ioutil"
    "encoding/json"
    "net/http/httputil"
)

// config for a vhost
type vHost struct {
    Origin          string                  `json:"origin"`
    VHosts          []string                `json:"vhosts"`
    Expire          int                     `json:"expire"`
    Proxy           *httputil.ReverseProxy
    ActiveRequests  *ActiveRequests
}

// Configuration settings for a log
type LogConfig struct {
    Type        string
    Location    string
    Format      string
    Verbose     bool
}

// Global Config structure
type Config struct {
    Server  []string
    Port    []int
    Cache   struct{
        Type    string
        Size    int
    }
    Log     []LogConfig
}

// hashmap of vhost to config
var vHosts map[string]*vHost
var config Config

// Load global config file
func loadConfig(path string) error {
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
    
    remote, err := url.Parse(config.Origin)
    if err != nil {
        return err
    }

    config.Proxy = httputil.NewSingleHostReverseProxy(remote)
    for _, v := range config.VHosts {
        vHosts[v] = &config
    }
    config.ActiveRequests = &ActiveRequests{
        Targets: make(map[string]*Target),
    }
    log.Println("Loaded config for", path)
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
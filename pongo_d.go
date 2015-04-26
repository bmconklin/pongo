package main

import(
    "os"
    "fmt"
    "log"
    "flag"
    "runtime"
    "./server"
    "encoding/json"
)

const version = "0.4.0"

// any flags passed in at runtime
var (
    v           = flag.Bool("v", false, "Display version number and quit")
    configDir   = flag.String("conf", "/etc/pongo/conf/pongo.conf", "location of config file")
    vhostDir    = flag.String("dir", "/etc/pongo/conf/vhosts", "root directory for vhost configs")
)

var config server.Config

// Load global config file
func loadConfig(path string) error {
    file, err := os.Open(path)
    if err != nil {
        return err
    }
    defer file.Close()

    jsonDecoder := json.NewDecoder(file)
    if err = jsonDecoder.Decode(&config); err != nil {
        return err
    }

    log.Println("Loaded Pongo config from", path)
    return nil
}

func init() {
    flag.Parse()
    if *v {
        fmt.Println("Pongo - Proxy on Go")
        fmt.Println("Version:", version)
        os.Exit(0)
    }
    runtime.GOMAXPROCS(runtime.NumCPU())
    if err := server.LoadConfig(*configDir); err != nil {
        log.Println(err)
        os.Exit(1)
    }
}

// start server
func main() {
    go func() {
        if err := server.StartProxy(); err != nil {
            log.Fatal(err)
            os.Exit(1)
        }
    } ()
    server.StartServer()
    fmt.Println("Server shut down, terminating this process now.")
}

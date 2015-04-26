package main

import(
    "os"
    "fmt"
    "log"
    "flag"
    "./client"
)

const version = "0.4.0"

var (
    count       = flag.Int("count", 0, "Number of concurrent chutes to redis")
    v           = flag.Bool("v", false, "Display version number and quit")
    vod         = flag.Bool("vod03", false, "Run for vod03 box")
    verbose     = flag.Bool("verbose", false, "Use verbose logging")
    conffile    = flag.String("config","/etc/pongo/conf/pongo_cli.conf","Override config file")
    cpuprofile  = flag.String("cpuprofile", "", "write cpu profile to file")
)

func init() {
    flag.Parse()
    if *v {
        fmt.Println("goNoSQL")
        fmt.Println("Version:", version)
        os.Exit(0)
    }
}

func main() {
    //get configs
    config, err := client.GetConfig(*conffile)
    if err != nil {
        log.Println(err)
        return
    }

    client.Connect(config)

    fmt.Println("Goodbye.")
}

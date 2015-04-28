package server

import(
    "os"
    "fmt"
    "log"
    "net"
    "sync"
    "bufio"
    "errors"
    "strings"
    "os/signal"
)

type Client struct {
    TCP         *net.TCPConn
    BWriter     *bufio.Writer
    BReader     *bufio.Reader
    DB          string
}

type Command struct {
    Name            string
    Usage           string
    Aliases         []string
    Subcommands     map[string]*Command
    Action          func([]string) (string, error)
}

var cmds map[string]*Command


func registerCommands() {
    cmds = make(map[string]*Command)

    cmds["find"] = &Command{
        "find",
        "View a vHost config matching the parameters",
        []string{},
        map[string]*Command{
            "pretty": &Command{
                "pretty",
                "Pretty Print the results",
                []string{},
                map[string]*Command{},
                func(context []string) (reply string, err error) {
                    if len(context) == 0 {
                        log.Println(vHosts)
                        for v, cfg := range vHosts {
                            // only display each one once
                            if cfg.VHosts[0] == v {
                                reply += cfg.PrettyString() + "\n"
                            }
                        }
                    } else {
                        for _, vHost := range context {
                            if _, ok := vHosts[vHost]; !ok {
                                continue
                            }
                            reply += vHosts[vHost].PrettyString() + "\n"
                        }
                    }
                    return
                },
            },
        },
        func(context []string) (reply string, err error) {
            if len(context) == 0 {
                for v, cfg := range vHosts {
                    // only display each one once
                    if cfg.VHosts[0] == v {
                        reply += cfg.String() + "\n"
                    }
                }
            } else {
                if _, ok := cmds["find"].Subcommands[context[0]]; ok {
                    return cmds["find"].Subcommands[context[0]].Action(context[1:])
                }
                for _, vHost := range context {
                    if _, ok := vHosts[vHost]; !ok {
                        continue
                    }
                    reply += vHosts[vHost].String() + "\n"
                }
            }
            return
        },
    }

    cmds["help"] = &Command{
        "help",
        "Display help information for commands",
        []string{
            "h",
        },
        map[string]*Command{},
        func(context []string) (reply string, err error) {
            if len(context) == 0 {
                for c, cmd := range cmds {
                    reply += c + "\t" + cmd.Usage + "\r\n"
                }
            } else {
                for _, cmd := range context {
                    if _, ok := cmds[cmd]; !ok {
                        continue
                    }
                    reply += cmd + "\t" + cmds[cmd].Usage + "\r\n"
                }
            }
            return
        },
    }
}

func handleCmd(cmd string, c *Client) (string, error) {
    cmd = strings.Trim(cmd, "\n")
    tokens := strings.Split(cmd, " ")
    if len(tokens) == 1 && tokens[0] == "" {
        return "", nil
    }
    if _, ok := cmds[tokens[0]]; !ok {
        return "", errors.New("Command not found.")
    }
    return cmds[tokens[0]].Action(tokens[1:])
}

func handleConnection(cmd string, c *Client) {    
    fmt.Fprintf(c.BWriter, "connection established\r\n\r\n")
}

func listenForClients(listener *net.TCPListener, k chan bool, wg *sync.WaitGroup) {
    defer wg.Done()

    var cWg sync.WaitGroup
    connCount := 0
    cK := make(chan bool, 1)

    for {
        conn, err := listener.AcceptTCP()
        if err != nil {
            log.Println(err)

            select {
            case <-k:
                for i := 0; i < connCount; i++ {
                    cK <- true
                }
                cWg.Wait()
                return
            default:
                continue
            }
        }

        // handle new connections
        c := &Client{
            conn,
            bufio.NewWriter(conn),
            bufio.NewReader(conn),
            "test",
        }
        connCount++
        cWg.Add(1)
        go startCommunication(c, cK, &cWg)
    }
}

func startCommunication(c *Client, k chan bool, wg *sync.WaitGroup) {
    defer wg.Done()
    defer c.TCP.Close()
    log.Println("Connection Established:", c.TCP.RemoteAddr().String())

    fmt.Fprintf(c.BWriter, "Connection established to " + c.TCP.LocalAddr().String() + "\n")
    c.BWriter.Flush()

    go func() {
        <-k
        fmt.Fprintf(c.BWriter, "Server is shutting down. Closing connection.\r\n")
        c.BWriter.Flush()
        c.TCP.Close()
    } ()

    for {
        cmd, err := c.BReader.ReadString('\n')
        if err != nil {
            log.Println("Connection closed from " + c.TCP.RemoteAddr().String())
            return
        }
        if len(cmd) == 0 {
            fmt.Fprintf(c.BWriter, "")
            continue
        }
        resp, err := handleCmd(cmd, c)
        resp = strings.Trim(resp, "\n")
        if err != nil {
            fmt.Fprintf(c.BWriter, err.Error() + "\r\n")
        } else {
            fmt.Fprintf(c.BWriter, resp + "\r\n")
        }
        c.BWriter.Flush()
    }
}

func StartServer() {
    
    // start syslog server
    fmt.Println("Starting Server.")

    tcpAddr := &net.TCPAddr{
        IP: net.ParseIP("127.0.0.1"),
        Port: 2042,
    }

    listener, err := net.ListenTCP("tcp", tcpAddr)
    if err != nil {
        log.Fatal(err)
        return
    }
    defer listener.Close()

    // Create a listener for the terminating signal
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, os.Kill)
    k := make(chan bool, 1)

    // register commands
    registerCommands()

    // start TCP listening loop
    var wg sync.WaitGroup
    wg.Add(1)
    go listenForClients(listener, k, &wg)

    // block until kill signal
    sig := <-c
    // Shutdown the server
    fmt.Println("Received signal:", sig, "Shutdown the server...")
    listener.Close()
    k <- true
    wg.Wait()
}

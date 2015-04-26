package server

import(
    "os"
    "fmt"
    "log"
    "net"
    "sync"
    "bufio"
    "os/signal"
)

type Client struct {
    TCP         *net.TCPConn
    BWriter     *bufio.Writer
    BReader     *bufio.Reader
    DB          string
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

func handleCmd(cmd string, c *Client) (string, error) {
    log.Print("Recieved command:", cmd)
    return "ok", nil
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

        resp, err := handleCmd(cmd, c)
        if err != nil {
            fmt.Fprintf(c.BWriter, err.Error() + "\n")
        } else {
            fmt.Fprintf(c.BWriter, resp + "\n")
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

    // start TCP listening loop
    var wg sync.WaitGroup
    wg.Add(1)
    go listenForClients(listener, k, &wg)

    // block until kill signal
    sig := <-c
    // Shutdown the server
    fmt.Println("Received signal: ", sig, "Shutdown the server...")
    listener.Close()
    k <- true
    wg.Wait()
}

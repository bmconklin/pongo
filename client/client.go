package client

import(
    "os"
    "fmt"
    "log"
    "net"
    "bufio"
    "strings"
)

func Connect(config *Config) {

    tcpAddr := &net.TCPAddr{
        IP: net.ParseIP("127.0.0.1"),
        Port: 2042,
    }

    conn, err := net.DialTCP("tcp", nil, tcpAddr)
    if err != nil {
        log.Fatal(err)
        return
    }
    defer conn.Close()

    br := bufio.NewReader(conn)
    bw := bufio.NewWriter(conn)

    startCommunication(br, bw)
}

func listener(br *bufio.Reader) string {
    resp := ""
    for {
        str, err := br.ReadString('\n')
        if err != nil {
            log.Fatal(err)
            return ""
        }
        resp += str
        if br.Buffered() == 0 {
            break
        }
    }
    return resp
}

func startCommunication(br *bufio.Reader, bw *bufio.Writer) {
    // listen for "connection established" from the server
    fmt.Print(listener(br))
    stdIn := bufio.NewReader(os.Stdin)
    
    for {
        fmt.Print("pongo_server> ")
        text, err := stdIn.ReadString('\n')
        if err != nil {
            log.Println(err)
        }
        if strings.HasPrefix(text, "exit") {
            return
        }
        fmt.Fprintf(bw, text)
        bw.Flush()
        fmt.Print(listener(br))
    }
}

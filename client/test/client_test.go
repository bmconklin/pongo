package client_test

import (
	"net"
	"bufio"
	. "../client"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client", func() {

    tcpAddr := &net.TCPAddr{
        IP: net.ParseIP("127.0.0.1"),
        Port: 2042,
    }

    var (
    	conn *net.TCPConn
    	br *bufio.Reader
    	bw *bufio.Writer
    )

    BeforeEach(func() {
    		var err error
		    conn, err = net.DialTCP("tcp", nil, tcpAddr)
		    if err != nil {
		        panic(err)
		    }

		    br = bufio.NewReader(conn)
		    bw = bufio.NewWriter(conn)
    	})

})

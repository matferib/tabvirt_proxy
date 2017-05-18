package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

var numClientes = 0

func chanFromConn(conn net.Conn) chan []byte {
	c := make(chan []byte)

	go func() {
		b := make([]byte, 1024)

		for {
			n, err := conn.Read(b)
			if n > 0 {
				res := make([]byte, n)
				// Copy the buffer so it doesn't get changed while read by the recipient.
				copy(res, b[:n])
				c <- res
			}
			if err != nil {
				c <- nil
				break
			}
		}
	}()

	return c
}

func pipe(conn1 net.Conn, conn2 net.Conn) {
	chan1 := chanFromConn(conn1)
	chan2 := chanFromConn(conn2)

	defer func() {
		log.Println("Fechando canais")
		conn1.Close()
		conn2.Close()
	}()

	for {
		select {
		case b1 := <-chan1:
			if b1 == nil {
				return
			} else {
				conn2.Write(b1)
			}
		case b2 := <-chan2:
			if b2 == nil {
				return
			} else {
				conn1.Write(b2)
			}
		}
	}
}

func conexaoCliente(cliente net.Conn, mestre net.Conn) {
	ln, err := net.Listen("tcp", ":11226")
	if err != nil {
		log.Println("Falha abrindo segundo socket mestre")
		return
	}

	_, err = mestre.Write([]byte("novo"))
	if err != nil {
		log.Println("Erro avisando mestre")
		return
	}
	mestre2, err := ln.Accept()
	if err != nil {
		log.Println("Erro recebendo nova conexao mestre")
		return
	}
	pipe(mestre2, cliente)
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello, world!")
}

func main() {
	http.HandleFunc("/", handler)
	go http.ListenAndServe(":8080", nil)
	log.Println("Iniciando")
	ln, err := net.Listen("tcp", ":11225")
	if err != nil {
		log.Println("Falha abrindo socket mestre")
		ln.Close()
		return
	}
	mestre, err := ln.Accept()
	if err != nil {
		log.Println("Falha ao receber conexao mestre")
		mestre.Close()
		return
	}

	ln, err = net.Listen("tcp", ":11227")
	if err != nil {
		log.Println("Falha abrindo socket clientes")
		return
	}
	numClientes = 0
	mestre.SetReadDeadline(time.Now())
	nop := []byte{}
	for {
		if _, err := mestre.Read(nop); err == io.EOF {
			log.Println("Server closed")
			break
		}
		conn, err := ln.Accept()
		if err != nil {
			log.Println("Falha recebendo conexao cliente")
			continue
		}
		numClientes++
		if numClientes > 1 {
			conn.Close()
			continue
		}
		go conexaoCliente(net.Conn(conn), net.Conn(mestre))
	}
}

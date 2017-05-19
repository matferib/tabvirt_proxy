package main

import (
	"fmt"
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
		numClientes--
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

func conexaoCliente(mestreClienteListener *net.TCPListener, mestre net.Conn, cliente net.Conn) error {
	log.Println("Novo cliente chegou, iniciando conexao")

	// Notifica mestre sobre novo cliente.
	_, err := mestre.Write([]byte("novo"))
	if err != nil {
		log.Println("Erro avisando mestre")
		return err
	}
	log.Println("Mestre notificado")

	// Aguarda nova conexao do mestre.
	mestreClienteListener.SetDeadline(time.Now().Add(5 * time.Second))
	mestreCliente, err := mestreClienteListener.Accept()
	if err != nil {
		cliente.Close()
		log.Println("Erro recebendo nova conexao mestre")
		return err
	}
	log.Println("Nova conexao com mestre ok")

	// Conecta os dois.
	go pipe(mestreCliente, cliente)
	log.Println("Pipe criado entre os dois")
	return nil
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello, world!")
}

func main() {
	// Cloud services.
	http.HandleFunc("/", handler)
	go http.ListenAndServe(":8080", nil)
	// End cloud services.

	log.Println("Iniciando")
	mestreListener, err := net.Listen("tcp", ":11225")
	if err != nil {
		log.Println("Falha abrindo socket mestre")
		mestreListener.Close()
		return
	}
	// Conexao principal do mestre, para notificacao de cliente novo.
	mestre, err := mestreListener.Accept()
	if err != nil {
		log.Println("Falha ao receber conexao mestre")
		mestreListener.Close()
		mestre.Close()
		return
	}
	defer func() { mestre.Close() }()
	log.Println("Mestre principal conectado")

	clienteListener, err := net.Listen("tcp", ":11227")
	if err != nil {
		log.Println("Falha abrindo socket clientes")
		return
	}
	defer func() { clienteListener.Close() }()

	numClientes = 0

	// Mestre precisa de uma conexao por cliente.
	mestreAddr, err := net.ResolveTCPAddr("tcp", ":11226")
	mestreClienteListener, err := net.ListenTCP("tcp", mestreAddr)
	if err != nil {
		log.Println("Falha abrindo segundo socket mestre")
		return
	}
	defer func() { mestreListener.Close() }()

	for {
		novoCliente, err := clienteListener.Accept()
		if err != nil {
			log.Println("Falha recebendo conexao cliente")
			continue
		}
		numClientes++
		if numClientes > 10 {
			log.Println("Numero maximo de clientes excedido")
			novoCliente.Close()
			continue
		}
		if conexaoCliente(mestreClienteListener, net.Conn(mestre), net.Conn(novoCliente)) != nil {
			log.Println("Mestre morreu")
			break
		}
	}
}

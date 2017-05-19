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

func pipe(writer net.Conn, reader net.Conn) {
	io.Copy(writer, reader)
	writer.Close()
	reader.Close()
}

func pipeDuplex(conn1 net.Conn, conn2 net.Conn) {
	go pipe(conn1, conn2)
	go pipe(conn2, conn1)
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
	go pipeDuplex(mestreCliente, cliente)
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

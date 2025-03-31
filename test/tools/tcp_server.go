package main

import (
	"fmt"
	"log"

	"golang.org/x/sys/unix"
)

// https://www.gnu.org/software/libc/manual/html_node/Sockets.html#Sockets
// https://www.gnu.org/software/libc/manual/html_node/Connections.html
// https://www.gnu.org/software/libc/manual/html_node/Server-Example.html
// https://www.binarytides.com/server-client-example-c-sockets-linux/
// https://www.tenouk.com/Module41.html
// http://users.pja.edu.pl/~jms/qnx/help/tcpip_4.25_en/prog_guide/sock_advanced_tut.html

var (
	// == SERVER ==
	PORT = 8080
	ADDR = [4]byte{127, 0, 0, 1}
	// ============
	LISTENBACKLOG = 100
	MAXMSGSIZE    = 8000
)

func main() {
	// func Socket(domain, typ, proto int) (fd int, err error)
	// * Socket will return the server socket file descriptor
	// Domaine type:
	// AF_INET  0x2 -> The Internet Protocol version 4 (IPv4) address family
	// AF_INET6 0x1E -> The Internet Protocol version 6 (IPv6) address family
	// Socket types:
	// SOCK_STREAM	1		     Stream (connection) socket for reliable, sequenced, connection oriented messages (think TCP)
	// SOCK_DGRAM	  2		     Datagram (conn.less) socket for connection-less, unreliable messages (think UDP or UNIX connections)
	// SOCK_RAW	    3		     Raw socket
	// Protocol type:
	// IPPROTO_IP -> Level IP
	serverFD, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, unix.IPPROTO_IP)
	if err != nil {
		log.Fatal("Socket: ", err)
	}

	serverAddr := &unix.SockaddrInet4{
		Port: PORT,
		Addr: ADDR,
	}

	// func Bind(fd int, sa Sockaddr) (err error)
	// * Bind will link a socket file descriptor to a socket address
	// Sockaddr is of type interface{}
	err = unix.Bind(serverFD, serverAddr)
	if err != nil {
		log.Fatal("Bind: ", err)
	}
	fmt.Printf("Server: Bound to addr: %d, port: %d\n", serverAddr.Addr, serverAddr.Port)

	// func Listen(sockfd int, backlog int) (err error)
	// * Listen will set sockfd as a passive socket ready to accept
	// incoming connection request
	err = unix.Listen(serverFD, LISTENBACKLOG)
	if err != nil {
		log.Fatal("Listen: ", err)
	}

	var activeFdSet unix.FdSet
	var tmpFdSet unix.FdSet
	var fdMax int
	FDZero(&activeFdSet)
	FDSet(serverFD, &activeFdSet)
	fdMax = serverFD

	fdAddr := FDAddrInit()

	for {
		// Store temporarily a copy of the current state of activeFdSet
		tmpFdSet = activeFdSet

		// func Select(int nfds, fd_set *FdSet, fd_set *FdSet, fd_set *FdSet, timeval *Timeval) error
		// * Select will disable in the FdSet copy the fd not yet ready to be read
		// -> ndfs : The select function checks only the first nfds file descriptors.
		// The usual thing is to pass FD_SETSIZE as the value of this argument.
		// -> fd_set : Data type represents file descriptor sets for the select function
		// -> timeval : The timeout specifies the maximum time to wait. If you pass
		// a null pointer for this argument, it means to block indefinitely until
		// one of the file descriptors is ready.
		// Specify zero as the time (a struct timeval containing all zeros)
		// if you want to find out which descriptors are ready without waiting if none are ready.
		// var timeval = unix.Timeval{
		// 	Sec:  0,
		// 	Usec: 0,
		// }
		n, err := unix.Select(fdMax+1, &tmpFdSet, nil, nil, nil)
		if err != nil {
			log.Fatal("Select: ", err)
		}

		fmt.Printf("Select $d", n)

		// Iterate over the fdSet and handle only the active file descriptors
		for fd := 0; fd < fdMax+1; fd++ {
			if FDIsSet(fd, &tmpFdSet) {
				if fd == serverFD {
					// func Accept(fd int) (nfd int, sa Sockaddr, err error)
					// * Accept extracts the first connection request on the queue of
					// pending connections for the listening socket, sockfd, creates a new
					// connected socket, and returns a new file descriptor referring
					// to that socket and the address of this socket.
					acceptedFD, acceptedAddr, err := unix.Accept(serverFD)
					if err != nil {
						log.Fatal("Accept: ", err)
					}
					// Add new socket file descriptor and address
					FDSet(acceptedFD, &activeFdSet)
					fdAddr.Set(acceptedFD, acceptedAddr)
					if acceptedFD > fdMax {
						fdMax = acceptedFD
					}
				} else {
					msg := make([]byte, MAXMSGSIZE)
					// func Recvfrom(fd int, msg []byte, flags int) (n int, from Sockaddr, err error)
					// * Recvfrom will read the client fd and store the data in msg
					// Do not forger to close the fd after
					sizeMsg, _, err := unix.Recvfrom(fd, msg, 0)
					if err != nil {
						fmt.Println("Recvfrom: ", err)
						FDClr(fd, &activeFdSet)
						unix.Close(fd)
						fdAddr.Clr(fd)
						continue
					}
					clientAddr := fdAddr.Get(fd)
					addrFrom := clientAddr.(*unix.SockaddrInet4)
					fmt.Printf("%d byte read from %d:%d on socket %d\n",
						sizeMsg, addrFrom.Addr, addrFrom.Port, fd)
					print("> Received message:\n" + string(msg) + "\n")
					response := []byte("We just received your message: " + string(msg))

					// func Sendmsg(dstFD int, p, oob []byte, to Sockaddr, flags int) error
					// * Sendmsg will send a message on the socket connection
					// dstFD is the destinataire file descriptor
					// msg is the content of the message
					// oob is the Out Of Band data
					// to is the receiver socket address
					// flags is the bitwise OR of zero or more of the following flags :
					// MSG_CONFIRM, MSG_DONTROUTE, MSG_DONTWAIT, MSG_EOR, MSG_MORE, MSG_NOSIGNAL, MSG_OOB
					err = unix.Sendmsg(
						fd,
						response,
						nil, clientAddr, unix.MSG_DONTWAIT)
					if err != nil {
						fmt.Println("Sendmsg: ", err)
					}
					print("< Response message:\n" + string(response) + "\n")
					// Clear socket file descriptor and address
					FDClr(fd, &activeFdSet)
					fdAddr.Clr(fd)
					// Close file descriptor
					unix.Close(fd)
				}
			}
		}
	}
}

// FdSet store the active FDs
// type unix.FdSet struct {
//     Bits [32]int32 // FD_SETSIZE = 1024 = 32x32
// }

// FDZero set to zero the fdSet
func FDZero(p *unix.FdSet) {
	p.Bits = [16]int64{}
}

// FDSet set a fd of fdSet
func FDSet(fd int, p *unix.FdSet) {
	p.Bits[fd/32] |= (1 << (uint(fd) % 32))
}

// FDClr clear a fd of fdSet
func FDClr(fd int, p *unix.FdSet) {
	p.Bits[fd/32] &^= (1 << (uint(fd) % 32))
}

// FDIsSet return true if fd is set
func FDIsSet(fd int, p *unix.FdSet) bool {
	return p.Bits[fd/32]&(1<<(uint(fd)%32)) != 0
}

// FDAddr is the type storing the sockaddr of each fd
type FDAddr map[int]unix.Sockaddr

// FDAddrInit init FDAddr with the size of FDSize
func FDAddrInit() *FDAddr {
	f := make(FDAddr, unix.FD_SETSIZE)
	return &f
}

// Get return the Sockaddr value of a given fd key
func (f *FDAddr) Get(fd int) unix.Sockaddr {
	return (*f)[fd]
}

// Set set the Sockaddr value of a given fd key
func (f *FDAddr) Set(fd int, value unix.Sockaddr) {
	(*f)[fd] = value
}

// Clr remove a given fd key in FDAddr
func (f *FDAddr) Clr(fd int) {
	delete(*f, fd)
}

/*/ //
 *  The MIT License (MIT)
 *
 *  Copyright (c) 2015 Bluek404
 *
 *  Permission is hereby granted, free of charge, to any person obtaining a copy
 *  of this software and associated documentation files (the "Software"), to deal
 *  in the Software without restriction, including without limitation the rights
 *  to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 *  copies of the Software, and to permit persons to whom the Software is
 *  furnished to do so, subject to the following conditions:
 *
 *  The above copyright notice and this permission notice shall be included in all
 *  copies or substantial portions of the Software.
 *
 *  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 *  IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 *  FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 *  AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 *  LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 *  OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 *  SOFTWARE.
/*/ //

package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
)

const proxyBuffSize = 128

const (
	CMD byte = iota
	PROXY
)

var (
	serverPort, proxyPort, key string
	exit                       = make(chan int)
	proxy                      = &Proxy{
		request:  make(chan bool, proxyBuffSize),
		response: make(chan net.Conn, proxyBuffSize),
		destroy:  make(chan bool),
	}
)

func init() {
	if len(os.Args) != 4 {
		fmt.Printf("用法:\n"+
			"%v 服务端口 代理通信端口 密钥\n", os.Args[0])
		os.Exit(2)
	}
	serverPort, proxyPort, key = os.Args[1], os.Args[2], os.Args[3]
}

func copy(dst, src net.Conn) {
	io.Copy(dst, src)
	dst.Close()
	src.Close()
}

// 与用户通信
func serve() {
	ln, err := net.Listen("tcp", serverPort)
	if err != nil {
		log.Println(err)
		exit <- 1
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go func() {
			log.Println("[+]:", conn.RemoteAddr())
			defer log.Println("[-]:", conn.RemoteAddr())
			pconn := proxy.getConn()
			if pconn == nil {
				// 代理客户端不在线
				conn.Close()
				return
			}
			go copy(conn, pconn)
			copy(pconn, conn)
		}()
	}
}

type Proxy struct {
	request  chan bool
	response chan net.Conn
	// 因为没用心跳包，
	// 所以只能靠用户请求来检测在线，
	// 但是如果还没有用户请求代理客户端就下线并重新上线，
	// 需要手动销毁上一个已关闭链接
	destroy chan bool
	online  bool
}

// 与代理客户端通信
func (p *Proxy) serve() {
	go func() { <-p.destroy }() // 接收首次销毁请求，看下面就明白了
	ln, err := net.Listen("tcp", proxyPort)
	if err != nil {
		log.Println(err)
		exit <- 1
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go func() {
			clientIP := conn.RemoteAddr().String()
			buf := make([]byte, 1024)
			n, err := conn.Read(buf)
			if err != nil {
				log.Println(err)
				conn.Close()
				return
			}
			k := string(buf[1:n])
			// TODO: 支持超长key和登录后使用session代替key
			if k != key {
				// 认证失败
				log.Println(conn.RemoteAddr(), "错误key:", k)
				conn.Close()
				return
			}
			// 首位字节为请求类型
			switch buf[0] {
			case CMD:
				log.Println("[+]代理客户端:", clientIP)
				defer log.Println("[-]代理客户端:", clientIP)
				p.destroy <- true // 请求销毁上一个链接
				defer conn.Close()
				p.online = true
				for {
					select {
					case <-p.destroy:
						// 新的代理客户端连线，销毁这个
						return
					case <-p.request:
						_, err = conn.Write([]byte{0})
						if err != nil {
							log.Println(err)
							p.response <- nil
							p.online = false
							// 因为现在已经销毁，所以需要开一个线程
							// 用于接收代理客户端上线时的销毁请求
							go func() { <-p.destroy }()
							return
						}
					}
				}
			case PROXY:
				// 发送用户真实IP给代理客户端
				// 首位字节为IP的字节长度
				ipByte := []byte(clientIP)
				l := byte(len(ipByte))
				_, err = conn.Write(append([]byte{l}, ipByte...))
				if err != nil {
					log.Println(err)
					return
				}
				p.response <- conn
			}
		}()
	}
}

func (p *Proxy) getConn() net.Conn {
	if !p.online {
		return nil
	}
	p.request <- true
	c := <-p.response
	if c == nil {
		return c
	}
	return c
}

func main() {
	go serve()
	go proxy.serve()
	os.Exit(<-exit)
}

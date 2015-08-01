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
	"time"
)

const (
	CMD byte = iota
	PROXY
)

var proxyServer, targetServer, key string

func init() {
	if len(os.Args) != 4 {
		fmt.Printf("用法:\n"+
			"%v 目标服务器 代理服务器 密钥\n", os.Args[0])
		os.Exit(2)
	}
	targetServer, proxyServer, key = os.Args[1], os.Args[2], os.Args[3]
}

func c2Server(sleep time.Duration) net.Conn {
	time.Sleep(time.Second * sleep)
	conn, err := net.Dial("tcp", proxyServer)
	if err != nil {
		log.Println("连接服务端失败:", err, "将于", int(sleep+1), "秒后重试")
		return c2Server(sleep + 1)
	}
	_, err = conn.Write(append([]byte{CMD}, []byte(key)...))
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	return conn
}

func copy(dst, src net.Conn) {
	io.Copy(dst, src)
	dst.Close()
	src.Close()
}

func proxy() {
	c, err := net.Dial("tcp", proxyServer)
	if err != nil {
		log.Println(err)
		return
	}
	_, err = c.Write(append([]byte{PROXY}, []byte(key)...))
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	// 读取用户IP长度
	buf := make([]byte, 1)
	n, err := c.Read(buf)
	if err != nil {
		log.Println(err)
		c.Close()
		return
	}
	if n != 1 {
		log.Println("读取用户IP长度失败")
		c.Close()
		return
	}
	ipLen := buf[0]
	buf = make([]byte, ipLen)
	// 读取用户真实IP
	n, err = c.Read(buf)
	if err != nil {
		log.Println(err)
		c.Close()
		return
	}
	if n != int(ipLen) {
		log.Println("用户IP长度错误")
		c.Close()
		return
	}
	clientIP := string(buf)
	// 与需代理的程序连接
	pc, err := net.Dial("tcp", targetServer)
	if err != nil {
		log.Println(err)
		c.Close()
		return
	}
	log.Printf("[+]: %v(%v)\n", clientIP, pc.LocalAddr())
	defer log.Printf("[-]: %v(%v)\n", clientIP, pc.LocalAddr())
	go copy(pc, c)
	copy(c, pc)
}

func main() {
	conn := c2Server(0)
	defer conn.Close()

	buf := make([]byte, 1)
	for {
		_, err := conn.Read(buf)
		if err != nil {
			log.Println(err)
			conn = c2Server(0)
		}
		go proxy()
	}
}

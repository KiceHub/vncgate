/***************************************
* VNC WebSocket反代
* Update: 2026-08-13
* Author: vncgate
* Applies: Linux
* Remark: 浏览器WS → Unix Domain Socket(优先)/TCP(回退) 中继
****************************************/
package main

/**********
* 包管理
***********/
import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

/**********
* 变量区
***********/
var (
	vncSockAddr string        //VNC Unix socket路径 (空=跳过)
	vncPortAddr string        //VNC TCP地址 (空=跳过)
	vncTimeout  time.Duration //VNC空闲超时时间
)

var vncUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

/**********
* 函数区
***********/
/* 尝试连接VNC后端 (UDS优先, TCP回退) */
func dialVNCBackend() (net.Conn, error) {
	// 每次dial超时较短, 保证回退响应快
	dialTimeout := 5 * time.Second

	// 优先尝试 Unix Domain Socket
	if vncSockAddr != "" {
		conn, err := net.DialTimeout("unix", vncSockAddr, dialTimeout)
		if err == nil {
			Info("VNC已连接: unix:%s", vncSockAddr)
			return conn, nil
		}
		Warn("VNC UDS不可用 %s: %v", vncSockAddr, err)
	}

	// 回退尝试 TCP
	if vncPortAddr != "" {
		conn, err := net.DialTimeout("tcp", vncPortAddr, dialTimeout)
		if err == nil {
			Info("VNC已连接: tcp:%s", vncPortAddr)
			return conn, nil
		}
		Warn("VNC TCP不可用 %s: %v", vncPortAddr, err)
	}

	return nil, fmt.Errorf("VNC后端不可达 (uds:%s, tcp:%s)", vncSockAddr, vncPortAddr)
}

/* WebSocket反代: /websockify → VNC后端 */
func handleWebsockify(w http.ResponseWriter, r *http.Request) {
	// 与浏览器建立WebSocket
	cliConn, err := vncUpgrader.Upgrade(w, r, nil)
	if err != nil {
		Error("VNC WS升级失败: %v", err)
		return
	}
	defer cliConn.Close()

	// 连接VNC后端 (UDS优先, TCP回退)
	backend, err := dialVNCBackend()
	if err != nil {
		Error("%v", err)
		cliConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr,
				fmt.Sprintf("VNC backend unreachable: %v", err)))
		return
	}
	defer backend.Close()

	done := make(chan struct{}, 2)

	// 浏览器 → 后端 (每次读到WS消息刷新deadline)
	go func() {
		defer func() { done <- struct{}{} }()
		for {
			cliConn.SetReadDeadline(time.Now().Add(vncTimeout))
			_, msg, err := cliConn.ReadMessage()
			if err != nil {
				return
			}
			backend.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if _, err := backend.Write(msg); err != nil {
				return
			}
		}
	}()

	// 后端 → 浏览器 (每次读到数据刷新deadline)
	go func() {
		defer func() { done <- struct{}{} }()
		buf := make([]byte, 8192)
		for {
			backend.SetReadDeadline(time.Now().Add(vncTimeout))
			n, err := backend.Read(buf)
			if err != nil {
				return
			}
			cliConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := cliConn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				return
			}
		}
	}()

	<-done
}

/**********
* End
***********/
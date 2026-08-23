/***************************************
* Web服务管理
* Update: 2026-08-13
* Author: vncgate
* Applies: Linux
* Remark: HTTP路由/模板渲染/监听启动
****************************************/
package main

/**********
* 包管理
***********/
import (
	"time"
	"embed"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"os"
)

/**********
* 变量区
***********/
//go:embed templates/*
//go:embed templates/novnc/**/*
var templateFiles embed.FS

var tmpl *template.Template

/**********
* 函数区
***********/
/* 启动HTTP服务 */
func startServer(cfg Config) {
	// 初始化系统用户
	if err := initSysUser(); err != nil {
		Fatal("系统用户初始化失败: %v", err)
		os.Exit(1)
	}

	// 设置VNC反代地址 (UDS优先, TCP回退)
	vncSockAddr = cfg.VNCSock
	vncPortAddr = ":" + cfg.VNCPort
	vncTimeout = 600 * time.Second //VNC空闲超时

	// 解析模板
	var err error
	tmpl, err = template.New("").Funcs(template.FuncMap{
		"SysUser": func() string { return sysUserName },
	}).ParseFS(templateFiles, "templates/*.html")
	if err != nil {
		Fatal("模板解析失败: %v", err)
		os.Exit(1)
	}

	go sessionCleaner()

	// 注册路由
	mux := http.NewServeMux()

	// noVNC静态文件 (挂载到根路径, 需认证)
	if novncFS, err := fs.Sub(templateFiles, "templates/novnc"); err == nil {
		novncFileServer := http.FileServer(http.FS(novncFS))
		mux.Handle("/vnc.html", authMiddleware(novncFileServer))
		mux.Handle("/defaults.json", authMiddleware(novncFileServer))
		mux.Handle("/mandatory.json", authMiddleware(novncFileServer))
		mux.Handle("/package.json", authMiddleware(novncFileServer))
		mux.Handle("/app/", authMiddleware(novncFileServer))
		mux.Handle("/core/", authMiddleware(novncFileServer))
		mux.Handle("/vendor/", authMiddleware(novncFileServer))
	}
	// ui.js 相对引用 -> /package.json
	mux.Handle("/app/package.json", authMiddleware(http.HandlerFunc(handlePackageJSON)))

	// 公开路由
	mux.HandleFunc("/favicon.ico", handleFavicon)
	mux.HandleFunc("/login", handleLoginPage)
	mux.HandleFunc("/api/login", handleLogin)
	mux.HandleFunc("/api/logout", handleLogout)

	// 需认证路由
	mux.Handle("/", authMiddleware(http.HandlerFunc(handleNoVNCPage)))
	mux.Handle("/websockify", authMiddleware(http.HandlerFunc(handleWebsockify)))

	// 启动监听
	listen := ":" + cfg.WebPort
	startTCP(listen, mux)
}

/* TCP监听启动 */
func startTCP(addr string, handler http.Handler) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		Fatal("TCP监听失败 %s: %v", addr, err)
		os.Exit(1)
	}
	Info("Web服务已启动: http://%s", addr)
	if err := http.Serve(listener, handler); err != nil {
		Error("HTTP服务错误: %v", err)
	}
}

/* 渲染模板 */
func renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		Error("模板渲染失败 %s: %v", name, err)
		http.Error(w, "内部错误", http.StatusInternalServerError)
	}
}

/**********
* 页面处理函数
***********/
/* 登录页 */
func handleLoginPage(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "login.html", nil)
}

/* favicon */
func handleFavicon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/x-icon")
	data, _ := templateFiles.ReadFile("templates/favicon.ico")
	w.Write(data)
}

/* noVNC版本信息 */
func handlePackageJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	data, _ := templateFiles.ReadFile("templates/novnc/package.json")
	w.Write(data)
}

/* noVNC页 (仅根路径, 其余路径404) */
func handleNoVNCPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	renderTemplate(w, "novnc.html", nil)
}

/**********
* End
***********/
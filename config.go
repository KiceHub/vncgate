/***************************************
* unix socket/VNC 设置
* Update: 2026-08-13
* Author: vncgate
* Applies: Linux
* Remark: 仅vncgate远程桌面 + 系统登录认证
****************************************/

package main

/**********
* 包管理
***********/
import (
	"flag"
	"strings"
)

/**********
* 变量区
***********/
type Config struct { //运行参数
	WebPort string //Web监听端口
	VNCSock string //VNC Unix socket路径
	VNCPort string //VNC TCP地址
	LogLevel string //日志等级
}

/**********
* 函数区
***********/
/* 解析命令行参数 */
func ParseFlags() Config {
	cfg := Config{}
	flag.StringVar(&cfg.WebPort, "web", "8080", "Web监听端口")
	flag.StringVar(&cfg.VNCSock, "sock", "/tmp/vnc.sock", "VNC Unix socket路径 (优先使用)")
	flag.StringVar(&cfg.VNCPort, "port", "5900", "VNC TCP端口 (回退)")
	flag.StringVar(&cfg.LogLevel, "l", "INFO", "日志等级")
	flag.Parse()

	// 补全端口格式
	cfg.WebPort = strings.TrimPrefix(cfg.WebPort, ":")
	if cfg.WebPort == "" {
		cfg.WebPort = "8080"
	}
	cfg.VNCPort = strings.TrimPrefix(cfg.VNCPort, ":")
	if cfg.VNCPort == "" {
		cfg.VNCPort = "5900"
	}
	if cfg.VNCSock == "" {
		cfg.VNCSock = "/tmp/vnc.sock"
	}

	return cfg
}

/**********
* End
***********/
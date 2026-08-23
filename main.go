/***************************************
* vncgate 远程桌面网关
* Update: 2026-08-13
* Author: vncgate
* Applies: Linux
* Dependent:
****************************************/

/***************************************
* 功能:
* 1. vncgate远程桌面 (UDS优先/TCP回退)
* 2. 系统非root用户登录认证
****************************************/

package main

/**********
* 包管理
***********/
import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

/**********
* 版本控制
***********/
var (
	VERSION     = "dev"
	BUILDTIME   = ""
	PKG_VERSION = ""
)

/**********
* 全局变量
***********/
var Lock *os.File //文件锁

/**********
* 本地函数
***********/
/* 退出前清理函数 */
func QuitMain() {
	Debug("程序正在退出...")
	if Lock != nil {
		syscall.Flock(int(Lock.Fd()), syscall.LOCK_UN)
	}
}

/* 异常退出函数 */
func QuitError(err error) {
	fmt.Printf("\n")
	Fatal("程序异常: %v", err)
	QuitMain()
	os.Exit(1)
}

/**********
* APP函数
***********/
func umain(cfg Config) {
	if err := LogInit(cfg.LogLevel); err != nil { //配置日志等级
		Fatal("日志等级错误: %v", err)
		os.Exit(1)
	}

	Info("Start %s(%s-%s)...", filepath.Base(os.Args[0]), VERSION, PKG_VERSION)
	Debug("Build Time: %s", BUILDTIME)

	// 启动Web服务
	startServer(cfg)

	select {} //阻塞协程
}

/**********
* 主函数
***********/
func main() {
	if os.Geteuid() != 0 {
		QuitError(fmt.Errorf("权限不足, 需要root权限运行!"))
	}
	lockFile := "/tmp/" + filepath.Base(os.Args[0]) + ".lock"
	var err error
	Lock, err = os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		QuitError(err)
	}
	err = syscall.Flock(int(Lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		QuitError(fmt.Errorf("已有进程在运行!"))
	}
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	cfg := ParseFlags()

	go umain(cfg) //启动主程序

	<-quit
	fmt.Printf("\n")
	Info("正在终止程序...")
	QuitMain()
}

/**********
* End
***********/
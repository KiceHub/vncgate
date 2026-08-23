/***************************************
* 日志管理
* Update: 2026-08-13
* Author: vncgate
* Applies: Linux
* Remark: 极简日志输出(独立程序内嵌)
****************************************/
package main

/**********
* 包管理
***********/
import (
	"fmt"
	"strings"
)

/**********
* 变量区
***********/
const ( //日志等级
	LOG_DEBUG uint8 = iota
	LOG_INFO
	LOG_WARN
	LOG_ERROR
	LOG_FATAL
	LOG_SILENT
)

var levelNames = [...]string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL", "SILENT"}
var currentLevel = LOG_INFO

/**********
* 函数区
***********/
/* 日志输出 */
func logout(level uint8, format string, args ...any) {
	if level < currentLevel {
		return
	}
	fmt.Printf("[%s] %s\n", levelNames[level], fmt.Sprintf(format, args...))
}

func Debug(format string, args ...any) { logout(LOG_DEBUG, format, args...) }
func Info(format string, args ...any)  { logout(LOG_INFO, format, args...) }
func Warn(format string, args ...any)  { logout(LOG_WARN, format, args...) }
func Error(format string, args ...any) { logout(LOG_ERROR, format, args...) }
func Fatal(format string, args ...any) { logout(LOG_FATAL, format, args...) }

/* 日志等级初始化 */
func LogInit(s string) error {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DEBUG":
		currentLevel = LOG_DEBUG
	case "INFO":
		currentLevel = LOG_INFO
	case "WARN":
		currentLevel = LOG_WARN
	case "ERROR":
		currentLevel = LOG_ERROR
	case "FATAL":
		currentLevel = LOG_FATAL
	case "SILENT":
		currentLevel = LOG_SILENT
	default:
		return fmt.Errorf("unknown log level: %s", s)
	}
	return nil
}

/**********
* End
***********/
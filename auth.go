/***************************************
* 认证管理
* Update: 2026-08-13
* Author: vncgate
* Applies: Linux
* Remark: 使用系统shadow验证非root用户
****************************************/
package main

/**********
* 包管理
***********/
import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/GehirnInc/crypt"
	_ "github.com/GehirnInc/crypt/sha256_crypt"
	_ "github.com/GehirnInc/crypt/sha512_crypt"
	yescrypt "github.com/openwall/yescrypt-go"
)

/**********
* 变量区
***********/
type Session struct { //会话结构体
	Token    string
	ExpireAt time.Time
}

type loginReq struct { //登录请求
	User string `json:"user"`
	Pass string `json:"pass"`
}

type loginResp struct { //登录响应
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

var (
	sessionMu   sync.RWMutex
	sessions    = make(map[string]*Session) //token -> session
	sysUserName string                      //系统非root用户名
	sysUserUID  int                         //系统非root用户UID

	loginFailMu    sync.Mutex
	loginFailCount int
	loginLockUntil time.Time
)

/**********
* 函数区
***********/
/* 初始化系统用户 (自动查找首个UID>=1000的非root用户) */
func initSysUser() error {
	users, err := readSystemUsers()
	if err != nil {
		return err
	}
	for _, u := range users {
		uid, _ := strconv.Atoi(u.Uid)
		if uid >= 1000 {
			sysUserName = u.Username
			sysUserUID = uid
			Info("自动检测系统用户: %s (UID:%d)", sysUserName, sysUserUID)
			return nil
		}
	}
	return fmt.Errorf("未找到系统非root用户(UID>=1000)")
}

/* 读取系统非root用户列表 */
func readSystemUsers() ([]*user.User, error) {
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return nil, err
	}
	var users []*user.User
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 7 {
			continue
		}
		uid, _ := strconv.Atoi(parts[2])
		if uid >= 1000 && parts[6] != "/usr/sbin/nologin" && parts[6] != "/bin/false" {
			users = append(users, &user.User{
				Username: parts[0],
				Uid:      parts[2],
				Gid:      parts[3],
				HomeDir:  parts[5],
			})
		}
	}
	return users, nil
}

/* 密码验证(对照/etc/shadow) */
func verifyPassword(username, password string) bool {
	hash := getShadowHash(username)
	if hash == "" {
		Warn("未找到用户 %s 的shadow条目", username)
		return false
	}

	// $y$ yescrypt格式 (纯Go)
	if strings.HasPrefix(hash, "$y$") {
		got, err := yescrypt.Hash([]byte(password), []byte(hash))
		return err == nil && string(got) == hash
	}

	// 使用crypt库验证 (支持 $1$ MD5 / $5$ SHA256 / $6$ SHA512)
	result := false
	func() {
		defer func() { recover() }()
		crypter := crypt.NewFromHash(hash)
		if crypter != nil {
			result = crypter.Verify(hash, []byte(password)) == nil
		}
	}()
	return result
}

/* 从/etc/shadow获取密码哈希 */
func getShadowHash(username string) string {
	data, err := os.ReadFile("/etc/shadow")
	if err != nil {
		Error("无法读取/etc/shadow: %v", err)
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, username+":") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 2 {
			continue
		}
		hash := parts[1]
		if hash == "!" || hash == "*" || hash == "" {
			return "" //账户已锁定或无密码
		}
		return hash
	}
	return ""
}

/* 生成随机token */
func generateToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

/* 创建新会话 */
func createSession() *Session {
	token, _ := generateToken()
	s := &Session{
		Token:    token,
		ExpireAt: time.Now().Add(24 * time.Hour),
	}
	sessionMu.Lock()
	sessions[token] = s
	sessionMu.Unlock()
	return s
}

/* 验证会话 */
func getSession(token string) *Session {
	sessionMu.RLock()
	defer sessionMu.RUnlock()
	s, ok := sessions[token]
	if !ok || time.Now().After(s.ExpireAt) {
		return nil
	}
	return s
}

/* 销毁会话 */
func deleteSession(token string) {
	sessionMu.Lock()
	delete(sessions, token)
	sessionMu.Unlock()
}

/* 定时清理过期会话 */
func sessionCleaner() {
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		sessionMu.Lock()
		for token, s := range sessions {
			if time.Now().After(s.ExpireAt) {
				delete(sessions, token)
			}
		}
		sessionMu.Unlock()
	}
}

/* 获取客户端IP */
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.Split(xff, ",")[0]
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if host != "" {
		return host
	}
	return r.RemoteAddr
}

/* 登录锁定检查, 返回剩余锁定时间(0=未锁定) */
func isLoginLocked() time.Duration {
	loginFailMu.Lock()
	defer loginFailMu.Unlock()
	if time.Now().Before(loginLockUntil) {
		return time.Until(loginLockUntil)
	}
	if loginFailCount >= 10 {
		loginFailCount = 0
	}
	return 0
}

/* 登录API处理 */
func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ip := clientIP(r)

	// 锁定检查
	if remaining := isLoginLocked(); remaining > 0 {
		s := remaining.Round(time.Second)
		Warn("登录锁定中, 拒绝 %s (剩余: %v)", ip, s)
		writeJSON(w, loginResp{OK: false, Message: fmt.Sprintf("登录已锁定, 请 %v 后重试", s)})
		return
	}

	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, loginResp{OK: false, Message: "请求格式错误"})
		return
	}

	// 只允许系统非root用户登录
	if req.User != sysUserName {
		Warn("登录失败 [%s]: 用户名 %s 错误", ip, req.User)
		loginFailMu.Lock()
		loginFailCount++
		if loginFailCount >= 10 {
			loginLockUntil = time.Now().Add(10 * time.Minute)
			Warn("登录锁定10分钟 (失败%d次)", loginFailCount)
		}
		loginFailMu.Unlock()
		writeJSON(w, loginResp{OK: false, Message: "用户名或密码错误"})
		return
	}

	if !verifyPassword(req.User, req.Pass) {
		Warn("登录失败 [%s]: 用户 %s 密码错误", ip, req.User)
		loginFailMu.Lock()
		loginFailCount++
		if loginFailCount >= 10 {
			loginLockUntil = time.Now().Add(10 * time.Minute)
			Warn("登录锁定10分钟 (失败%d次)", loginFailCount)
		}
		loginFailMu.Unlock()
		writeJSON(w, loginResp{OK: false, Message: "用户名或密码错误"})
		return
	}

	// 成功登录: 重置失败计数
	loginFailMu.Lock()
	loginFailCount = 0
	loginLockUntil = time.Time{}
	loginFailMu.Unlock()

	s := createSession()
	http.SetCookie(w, &http.Cookie{
		Name:     "ssweb_session",
		Value:    s.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})

	Info("登录成功 [%s]: 用户 %s", ip, req.User)
	writeJSON(w, loginResp{OK: true})
}

/* 登出API处理 */
func handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("ssweb_session"); err == nil {
		deleteSession(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:   "ssweb_session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

/* 认证中间件 */
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("ssweb_session")
		if err != nil {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"ok":false,"msg":"unauthorized"}`))
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		s := getSession(cookie.Value)
		if s == nil {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"ok":false,"msg":"unauthorized"}`))
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

/* JSON响应辅助函数 */
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}


/**********
* End
***********/
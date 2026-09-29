// Command pmjira is a single-binary personal project management web app.
// Double-click the built executable (or run it) to start the server and open
// the UI in your browser. Data is stored in a local SQLite database.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"

	"pmjira/internal/store"
	"pmjira/internal/web"
)

const defaultPort = 8765

// guiBuild is set to "true" via -ldflags "-X main.guiBuild=true" for the
// no-console (windowsgui) Windows build, so logs are redirected to a file.
// (linker -X only supports string vars, hence the string sentinel.)
var guiBuild = "false"

func main() {
	addr := flag.String("addr", fmt.Sprintf(":%d", defaultPort), "监听地址（:端口）")
	dbFlag := flag.String("db", "", "SQLite 数据库路径（默认为程序同目录下的 pmjira.db）")
	configFlag := flag.String("config", "", "配置文件路径（默认为程序同目录下的 jira_config.json）")
	noBrowser := flag.Bool("no-browser", false, "不要自动打开浏览器")
	flag.Parse()

	dbPath := *dbFlag
	if dbPath == "" {
		dbPath = defaultDBPath()
	}

	configPath := *configFlag
	if configPath == "" {
		configPath = web.DefaultConfigPath()
	}

	// For the no-console build there is no visible terminal, so tee logs to a
	// file next to the database for debugging.
	if guiBuild == "true" {
		if logPath := filepath.Join(filepath.Dir(dbPath), "pmjira.log"); logPath != "" {
			if f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
				log.SetOutput(f)
			}
		}
	}

	log.Printf("PM Jira 启动中… 数据库：%s", dbPath)
	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("打开数据库失败：%v", err)
	}
	defer st.Close()

	cfg := web.NewConfigProvider(configPath)
	srv := web.New(st, cfg)

	ln, err := listenWithFallback(*addr)
	if err != nil {
		log.Fatalf("无法监听：%v", err)
	}
	url := fmt.Sprintf("http://localhost:%d/", portOf(ln.Addr().String()))
	log.Printf("✓ 服务已就绪：%s", url)
	log.Printf("  按 Ctrl+C 停止。")

	if !*noBrowser {
		go func() {
			time.Sleep(300 * time.Millisecond) // let the server accept connections
			openBrowser(url)
		}()
	}

	httpSrv := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	select {
	case <-ctx.Done():
		log.Println("正在停止…（Ctrl+C / 关闭）")
	case <-srv.ShutdownCh():
		log.Println("正在停止…（来自网页的停止请求）")
	case err := <-errCh:
		if err != nil {
			log.Fatalf("服务异常退出：%v", err)
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
}

// defaultDBPath places the database next to the executable so it is easy to
// find and back up. Falls back to the current working directory.
func defaultDBPath() string {
	exe, err := os.Executable()
	if err == nil {
		return filepath.Join(filepath.Dir(exe), "pmjira.db")
	}
	return "pmjira.db"
}

// listenWithFallback tries the requested address; if it is a fixed port that is
// already taken, falls back to an OS-chosen free port so the app still starts.
func listenWithFallback(addr string) (net.Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err == nil {
		return ln, nil
	}
	if isAddrInUse(err) {
		log.Printf("默认端口被占用，自动改用空闲端口")
		return net.Listen("tcp", "127.0.0.1:0")
	}
	return nil, err
}

// isAddrInUse returns true for the common "address already in use" errors.
func isAddrInUse(err error) bool {
	type interfaceErr interface{ Temporary() bool }
	if oe, ok := err.(*net.OpError); ok {
		if se, ok := oe.Err.(interface{ Error() string }); ok {
			msg := se.Error()
			return msg == "bind: address already in use" || msg == "address already in use"
		}
	}
	return false
}

// portOf extracts the TCP port from an Addr.String() like "127.0.0.1:8765".
func portOf(s string) int {
	_, p, err := net.SplitHostPort(s)
	if err != nil {
		return defaultPort
	}
	n := 0
	for _, c := range p {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// openBrowser opens the default browser to the URL (best effort).
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	_ = cmd.Start()
}

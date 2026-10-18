// Command dropcatch tracks product prices across Singapore shops and emails
// you when one drops well below its typical price.
//
//	dropcatch [serve]        run the dashboard and the scheduler
//	dropcatch install-agent  start dropcatch at login with launchd
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/rand/v2"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"text/template"
	"time"

	"dropcatch/internal/browser"
	"dropcatch/internal/checker"
	"dropcatch/internal/notify"
	"dropcatch/internal/reader"
	"dropcatch/internal/reader/amazon"
	"dropcatch/internal/reader/courts"
	"dropcatch/internal/reader/lazada"
	"dropcatch/internal/reader/shopee"
	"dropcatch/internal/scheduler"
	"dropcatch/internal/server"
	"dropcatch/internal/store"
	"dropcatch/web"
)

const agentLabel = "com.dropcatch.agent"

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "dropcatch-data"
	}
	return filepath.Join(home, "Library", "Application Support", "Dropcatch")
}

func main() {
	cmd := "serve"
	args := os.Args[1:]
	if len(args) > 0 && (args[0] == "serve" || args[0] == "install-agent") {
		cmd, args = args[0], args[1:]
	}
	fl := flag.NewFlagSet(cmd, flag.ExitOnError)
	addr := fl.String("addr", "127.0.0.1:8080", "listen address")
	data := fl.String("data", defaultDataDir(), "data directory")
	chrome := fl.String("chrome", browser.DefaultChromePath, "Chrome binary")
	profile := fl.String("profile", "", "Chrome profile directory (default <data>/chrome-profile)")
	_ = fl.Parse(args)
	if *profile == "" {
		*profile = filepath.Join(*data, "chrome-profile")
	}

	var err error
	switch cmd {
	case "install-agent":
		err = installAgent(*addr, *data)
	default:
		err = serve(*addr, *data, *chrome, *profile)
	}
	if err != nil {
		log.Fatal(err)
	}
}

func randomPause(time.Duration) {
	time.Sleep(time.Duration(5+rand.IntN(16)) * time.Second)
}

func serve(addr, data, chromePath, profile string) error {
	if err := os.MkdirAll(filepath.Join(data, "logs"), 0o755); err != nil {
		return err
	}
	st, err := store.Open(filepath.Join(data, "dropcatch.db"))
	if err != nil {
		return err
	}
	defer st.Close()

	reader.Register(&shopee.Reader{Sleep: randomPause})
	reader.Register(&lazada.Reader{Sleep: randomPause})
	reader.Register(amazon.New())
	reader.Register(courts.New())

	chrome := browser.New(browser.Config{ChromePath: chromePath, ProfileDir: profile})
	defer chrome.Close()
	// Launch Chrome now. It takes focus once, at startup, instead of during
	// a check while you're typing.
	go func() {
		if err := chrome.Start(); err != nil {
			log.Printf("chrome: %v", err)
		}
	}()
	// DROPCATCH_RESEND_URL points email at a local sink for end to end tests.
	mailer := &notify.SettingsMailer{Store: st, BaseURL: os.Getenv("DROPCATCH_RESEND_URL")}
	notices := &notify.Notices{
		Store: st, Mailer: mailer, Now: time.Now,
		To: func(ctx context.Context) (string, error) {
			s, err := st.GetSettings(ctx)
			return s.Email, err
		},
	}
	chk := &checker.Checker{Store: st, Browser: chrome, Mailer: mailer, Notices: notices, Now: time.Now, Sleep: randomPause}
	sched := &scheduler.Scheduler{Checker: chk, Store: st, Notices: notices, Now: time.Now}

	static, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           server.New(server.Deps{Store: st, Checker: chk, Mailer: mailer, Login: chrome, Now: time.Now}, static),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go sched.Run(ctx)
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()

	log.Printf("dropcatch on http://%s (data in %s)", addr, data)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

var plist = template.Must(template.New("plist").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>{{.Label}}</string>
  <key>ProgramArguments</key>
  <array>
    <string>{{.Binary}}</string>
    <string>serve</string>
    <string>-addr</string><string>{{.Addr}}</string>
    <string>-data</string><string>{{.Data}}</string>
  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>{{.Log}}</string>
  <key>StandardErrorPath</key><string>{{.Log}}</string>
</dict>
</plist>
`))

// installBinary copies the program to <data>/bin/dropcatch and returns that
// path. macOS guards ~/Documents behind a privacy prompt, and a background
// service started from there waits on that prompt again after every
// rebuild. Application Support has no such prompt.
func installBinary(src, data string) (string, error) {
	dir := filepath.Join(data, "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(dir, "dropcatch-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name()) // no-op after the rename
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return "", err
	}
	// Rename over the old copy so a running service keeps its open file.
	dst := filepath.Join(dir, "dropcatch")
	return dst, os.Rename(tmp.Name(), dst)
}

func installAgent(addr, data string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if self, err = filepath.EvalSymlinks(self); err != nil {
		return err
	}
	bin, err := installBinary(self, data)
	if err != nil {
		return fmt.Errorf("copy program: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(data, "logs"), 0o755); err != nil {
		return err
	}
	path := filepath.Join(home, "Library", "LaunchAgents", agentLabel+".plist")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = plist.Execute(f, map[string]string{
		"Label": agentLabel, "Binary": bin, "Addr": addr, "Data": data,
		"Log": filepath.Join(data, "logs", "dropcatch.log"),
	})
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}

	domain := "gui/" + strconv.Itoa(os.Getuid())
	service := domain + "/" + agentLabel
	_ = exec.Command("launchctl", "bootout", service).Run() // fine if it wasn't loaded
	// bootout returns before the old Dropcatch has exited (it closes Chrome
	// first), and bootstrapping too early fails with an I/O error.
	for i := 0; i < 30 && exec.Command("launchctl", "print", service).Run() == nil; i++ {
		time.Sleep(500 * time.Millisecond)
	}
	var out []byte
	for attempt := 0; attempt < 3; attempt++ {
		if out, err = exec.Command("launchctl", "bootstrap", domain, path).CombinedOutput(); err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return fmt.Errorf("launchctl bootstrap: %v: %s", err, out)
	}
	fmt.Printf("Dropcatch starts at login now. Open http://%s\nLaunch agent: %s\n", addr, path)
	return nil
}

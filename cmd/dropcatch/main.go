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

func installAgent(addr, data string) error {
	bin, err := os.Executable()
	if err != nil {
		return err
	}
	if bin, err = filepath.EvalSymlinks(bin); err != nil {
		return err
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
	_ = exec.Command("launchctl", "bootout", domain+"/"+agentLabel).Run() // fine if it wasn't loaded
	if out, err := exec.Command("launchctl", "bootstrap", domain, path).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootstrap: %v: %s", err, out)
	}
	fmt.Printf("Dropcatch starts at login now. Open http://%s\nLaunch agent: %s\n", addr, path)
	return nil
}

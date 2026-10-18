// Command capture saves what a product page shows, for writing site readers.
//
//	go run ./cmd/capture -profile DIR -login URL
//	go run ./cmd/capture -profile DIR -out DIR [-capture a,b] URL
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"dropcatch/internal/browser"
)

func main() {
	profile := flag.String("profile", "", "chrome profile dir")
	out := flag.String("out", "capture", "output dir")
	capture := flag.String("capture", "", "comma separated response URL substrings to save")
	login := flag.String("login", "", "open this URL visibly and wait until you close the window")
	settle := flag.Duration("settle", 8*time.Second, "wait after load")
	timeout := flag.Duration("timeout", 90*time.Second, "give up on the page after this")
	flag.Parse()
	if *profile == "" {
		log.Fatal("-profile is required")
	}
	c := browser.New(browser.Config{ProfileDir: *profile, Settle: *settle, Timeout: *timeout})
	ctx := context.Background()

	if *login != "" {
		if err := c.OpenVisible(ctx, *login, nil, 10*time.Minute); err != nil {
			log.Fatal(err)
		}
		fmt.Println("window closed")
		return
	}
	if flag.NArg() != 1 {
		log.Fatal("give one URL")
	}
	var subs []string
	if *capture != "" {
		subs = strings.Split(*capture, ",")
	}
	page, err := c.Load(ctx, flag.Arg(0), subs)
	if err != nil {
		log.Fatal(err)
	}
	must(os.MkdirAll(*out, 0o755))
	must(os.WriteFile(filepath.Join(*out, "page.html"), []byte(page.HTML), 0o644))
	must(os.WriteFile(filepath.Join(*out, "final_url.txt"), []byte(page.FinalURL+"\n"), 0o644))
	unsafe := regexp.MustCompile(`[^A-Za-z0-9._-]+`)
	for u, body := range page.Responses {
		name := unsafe.ReplaceAllString(strings.SplitN(strings.TrimPrefix(u, "https://"), "?", 2)[0], "_")
		if len(name) > 100 {
			name = name[:100]
		}
		must(os.WriteFile(filepath.Join(*out, name+".json"), body, 0o644))
	}
	fmt.Printf("final url: %s\nhtml: %d bytes, responses: %d\n", page.FinalURL, len(page.HTML), len(page.Responses))
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

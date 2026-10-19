// Command scriptedmodel serves the client tests' scripted model on a loopback
// address, for driving Goose or VS Code by hand against mm-mcp the way the
// client tests do (ADR-036). The tests start their own; this is for watching
// a client do what a test does.
//
//	go run ./tools/scriptedmodel [-listen 127.0.0.1:18080]
//
// Point the client's OpenAI-compatible provider at http://<listen>/v1, ask for
// the model scripted-model, name mm-mcp's server mattermost, and say, for
// example, SCRIPT show {"kind":"thread","post_id":"<id>"}.
package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/vriesdemichael/mm-mcp/internal/scriptedmodel"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:18080", "the loopback address to serve on")
	flag.Parse()
	host, _, err := net.SplitHostPort(*listen)
	if err != nil || !net.ParseIP(host).IsLoopback() {
		fmt.Fprintf(os.Stderr, "scriptedmodel: %s is not a loopback address\n", *listen)
		os.Exit(2)
	}
	fmt.Fprintf(os.Stderr, "scriptedmodel: serving %s at http://%s/v1\n", scriptedmodel.Name, *listen)
	server := &http.Server{Addr: *listen, Handler: (&scriptedmodel.Model{}).Handler(), ReadHeaderTimeout: 10 * time.Second}
	if err := server.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, "scriptedmodel:", err)
		os.Exit(1)
	}
}

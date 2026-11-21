//go:build browser

package login

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// A sign-in through single sign-on leaves the server for another site, the
// identity provider's, and comes back, and only then does the server set its
// session cookie. mm-mcp does nothing at the provider: it opens the window and
// waits for the cookie (Start, then WaitForToken, as `mm-mcp login` calls
// them). So what is tested here is the trip, in every browser on this machine:
// that the wait sees a cookie set at the end of one, however the browser gets
// there and back.
//
// The two sites are this test's own, and neither is Mattermost (ADR-005): one
// on 127.0.0.1, which stands where the session is for, and one under the name
// localhost, another site to a browser, which stands where the provider is.
// The first sets the cookie mm-mcp waits for, by its name, at the end of a
// trip. Whether Mattermost ends a single sign-on that way is not claimed here;
// a login through a real provider is still checked by hand.

// The trips, by how the browser gets back from the other site.
const (
	// byRedirects is there and back by redirects alone, as OpenID Connect and
	// OAuth go.
	byRedirects = "redirects"
	// byPostedForm is back in a form the provider's page posts, as SAML goes.
	byPostedForm = "posted-form"
	// inAPopup is a sign-in the login page opens in a window of its own, which
	// closes when it is done.
	inAPopup = "popup"
	// afterAStay is a person who stays at the provider a while, as for a second
	// factor, before going back.
	afterAStay = "stay"
)

// stay is how long the provider's page keeps the person in afterAStay: several
// looks at the cookies long.
const stay = 3 * time.Second

// signIn is the two sites, and what was asked of them.
type signIn struct {
	// server is where the session is for, on 127.0.0.1.
	server *httptest.Server
	// provider is the other site's address, under the name localhost.
	provider string
	// tokens is the session the server sets at the end of each trip.
	tokens map[string]string
	// decoy is a cookie of the provider's own, under the session cookie's name.
	decoy string

	mu sync.Mutex
	// codes is the trip each code the provider gave out is for; the server sets
	// a session only for a code, so only a trip through the provider ends in one.
	codes map[string]string
	asked []string
}

func random() string {
	raw := make([]byte, 8)
	_, _ = rand.Read(raw)
	return hex.EncodeToString(raw)
}

func (s *signIn) note(what string) {
	s.mu.Lock()
	s.asked = append(s.asked, what)
	s.mu.Unlock()
}

// count is how often something was asked for.
func (s *signIn) count(what string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, asked := range s.asked {
		if asked == what {
			n++
		}
	}
	return n
}

func (s *signIn) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.asked) == 0 {
		return "nothing"
	}
	return strings.Join(s.asked, ", ")
}

// document writes a page of this test's own.
func document(w http.ResponseWriter, title, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, "<!doctype html><title>%s</title><p>%s</p>%s", html.EscapeString(title), html.EscapeString(title), body)
}

// newSignIn starts the two sites.
func newSignIn(t *testing.T) *signIn {
	t.Helper()
	s := &signIn{tokens: map[string]string{}, codes: map[string]string{}, decoy: random()}
	for _, trip := range []string{byRedirects, byPostedForm, inAPopup, afterAStay} {
		s.tokens[trip] = random()
	}

	server := http.NewServeMux()
	server.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { document(w, "ready", "") })
	// The login page: no session yet, and a way to the provider that waits for
	// a press, as a login page's button does. An earlier trip's session is
	// ended, so each trip starts as a login does.
	server.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		trip := r.URL.Query().Get("trip")
		s.note("login " + trip)
		http.SetCookie(w, &http.Cookie{Name: CookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
		to := s.provider + "/authorize?trip=" + trip
		// After the press has been answered, so the answer is not lost to the
		// page going away.
		document(w, "login", fmt.Sprintf(`<script>
function leave() { setTimeout(() => location.assign(%q), 100) }
function popup() { window.open(%q) }
</script>`, to, to))
	})
	// Where the provider sends the browser back to, by address or in a posted
	// form: the session is set here, for a code the provider gave.
	server.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		trip, known := s.codes[r.FormValue("code")]
		delete(s.codes, r.FormValue("code"))
		s.mu.Unlock()
		if !known {
			s.note("callback without a code the provider gave")
			http.Error(w, "no such code", http.StatusBadRequest)
			return
		}
		s.note("callback " + trip + " " + r.Method)
		http.SetCookie(w, &http.Cookie{Name: CookieName, Value: s.tokens[trip], Path: "/", HttpOnly: true})
		http.Redirect(w, r, "/done?trip="+trip, http.StatusSeeOther)
	})
	server.HandleFunc("/done", func(w http.ResponseWriter, r *http.Request) {
		body := ""
		if r.URL.Query().Get("trip") == inAPopup {
			body = "<script>window.close()</script>"
		}
		document(w, "done", body)
	})
	s.server = httptest.NewServer(server)
	t.Cleanup(s.server.Close)

	provider := http.NewServeMux()
	port := ""
	provider.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		// Under any other name the two would be one site to a browser, and the
		// trip no trip to another.
		if r.Host != "localhost:"+port {
			s.note("provider asked as " + r.Host)
			http.Error(w, "the provider is at localhost", http.StatusBadRequest)
			return
		}
		trip := r.URL.Query().Get("trip")
		s.note("provider " + trip)
		code := random()
		s.mu.Lock()
		s.codes[code] = trip
		s.mu.Unlock()
		// The provider's own cookie under the same name, set before the
		// server's: it is not the server's session, and is never taken for it.
		http.SetCookie(w, &http.Cookie{Name: CookieName, Value: s.decoy, Path: "/", HttpOnly: true})
		back := s.server.URL + "/callback"
		switch trip {
		case byPostedForm:
			document(w, "provider", fmt.Sprintf(`<form method="post" action="%s"><input type="hidden" name="code" value="%s"></form>
<script>document.forms[0].submit()</script>`, html.EscapeString(back), code))
		case afterAStay:
			document(w, "provider", fmt.Sprintf(`<script>setTimeout(() => location.assign(%q), %d)</script>`, back+"?code="+code, stay.Milliseconds()))
		default:
			http.Redirect(w, r, back+"?code="+code, http.StatusFound)
		}
	})
	// A browser reads localhost as this machine's own address, of either
	// kind, so the provider listens on both where the machine has both.
	listen := &net.ListenConfig{}
	four, err := listen.Listen(t.Context(), "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening for the provider: %v", err)
	}
	_, port, _ = net.SplitHostPort(four.Addr().String())
	listeners := []net.Listener{four}
	if six, err := listen.Listen(t.Context(), "tcp6", net.JoinHostPort("::1", port)); err == nil {
		listeners = append(listeners, six)
	}
	for _, listener := range listeners {
		serving := &http.Server{Handler: provider, ReadHeaderTimeout: 10 * time.Second}
		go func() { _ = serving.Serve(listener) }()
		t.Cleanup(func() { _ = serving.Close() })
	}
	s.provider = "http://localhost:" + port
	return s
}

// press runs a script in the window's page as a person's press does: a
// browser opens a window for a page only on one.
func press(ctx context.Context, window Window, script string) error {
	var failed string
	var err error
	switch w := window.(type) {
	case *chromiumWindow:
		var evaluated struct {
			ExceptionDetails *struct {
				Text string `json:"text"`
			} `json:"exceptionDetails"`
		}
		err = w.onPage(ctx, func(session string) error {
			return w.call(ctx, session, "Runtime.evaluate", map[string]any{"expression": script, "userGesture": true}, &evaluated)
		})
		if evaluated.ExceptionDetails != nil {
			failed = evaluated.ExceptionDetails.Text
		}
	case *firefoxWindow:
		var evaluated struct {
			Type string `json:"type"`
		}
		err = w.call(ctx, "script.evaluate", map[string]any{
			"expression": script, "awaitPromise": false, "userActivation": true, "target": map[string]any{"context": w.context},
		}, &evaluated)
		if evaluated.Type == "exception" {
			failed = "an exception"
		}
	default:
		return fmt.Errorf("no way to press in a %T", window)
	}
	if err == nil && failed != "" {
		err = fmt.Errorf("the script failed: %s", failed)
	}
	return err
}

// untilSignedOut waits for the window to hold no session for the server, as
// at the start of a login.
func untilSignedOut(ctx context.Context, t *testing.T, window Window, address string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		cookies, err := window.Cookies(ctx)
		if err != nil {
			t.Fatalf("reading the cookies: %v", err)
		}
		if _, held := SessionToken(cookies, address); !held {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the login page did not end the session an earlier trip left; the browser holds %+v", cookies)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// Every browser mm-mcp logs in with waits out a sign-in that goes through
// another site, whichever way it comes back: by redirects, in a posted form,
// in a popup that closes, or after a stay at the other site. The wait answers
// with the session the server set at the end of that trip, never the other
// site's cookie of the same name, and the login page is opened once: nothing
// sends the window back to it while the person is away.
func TestEveryBrowserWaitsOutASignInThroughAnotherSite(t *testing.T) {
	for _, browser := range installed(t) {
		t.Run(browser.Name+" "+browser.Path, func(t *testing.T) {
			s := newSignIn(t)
			ctx, cancel := context.WithTimeout(t.Context(), 4*time.Minute)
			defer cancel()
			window, err := Start(ctx, browser, s.server.URL+"/", Options{Headless: true, StartTimeout: time.Minute})
			if err != nil {
				t.Fatalf("starting it: %v", err)
			}
			defer window.Close()

			for _, trip := range []string{byRedirects, byPostedForm, inAPopup, afterAStay} {
				t.Run(trip, func(t *testing.T) {
					if err := window.Navigate(ctx, s.server.URL+"/login?trip="+trip); err != nil {
						t.Fatalf("opening the login page: %v", err)
					}
					untilSignedOut(ctx, t, window, s.server.URL)
					script := "leave()"
					if trip == inAPopup {
						script = "popup()"
					}
					if err := press(ctx, window, script); err != nil {
						t.Fatalf("pressing the login page's button: %v", err)
					}

					waiting, stop := context.WithTimeout(ctx, 45*time.Second)
					defer stop()
					began := time.Now()
					token, err := window.WaitForToken(waiting, s.server.URL)
					switch {
					case err != nil:
						t.Fatalf("waiting for the session: %v; the sites were asked for %s, and the browser's pages are %s", err, s, openPages(window))
					case token == s.decoy:
						t.Fatalf("the wait took the other site's cookie for the server's session; the sites were asked for %s", s)
					case token != s.tokens[trip]:
						t.Fatalf("the wait answered %q, not the session this trip ended in, %q; the sites were asked for %s", token, s.tokens[trip], s)
					}
					if s.count("provider "+trip) != 1 {
						t.Errorf("the trip did not go through the other site once; the sites were asked for %s", s)
					}
					if opened := s.count("login " + trip); opened != 1 {
						t.Errorf("the login page was opened %d times, not once; the sites were asked for %s", opened, s)
					}
					method := http.MethodGet
					if trip == byPostedForm {
						method = http.MethodPost
					}
					if s.count("callback "+trip+" "+method) != 1 {
						t.Errorf("the browser did not come back by %s; the sites were asked for %s", method, s)
					}
					if trip == afterAStay && time.Since(began) < stay {
						t.Errorf("the session came after %s, before the stay of %s was over", time.Since(began).Round(time.Millisecond), stay)
					}
				})
			}
		})
	}
}

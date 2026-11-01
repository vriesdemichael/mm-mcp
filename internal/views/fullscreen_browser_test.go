//go:build views

package views_test

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"slices"
	"testing"
)

// Fullscreen is the host's to give and take (ADR-022): the view declares that it
// can, fills the frame it is given, and asks for it only for an opened image.

// tallImage is a thread's pictures, with a whole image taller than any frame.
func tallImage(t *testing.T) map[string]any {
	t.Helper()
	var tall bytes.Buffer
	if err := png.Encode(&tall, image.NewRGBA(image.Rect(0, 0, 1000, 3000))); err != nil {
		t.Fatal(err)
	}
	resources := pictures()
	resources[base+"/files/f1"] = map[string]any{"mimeType": "image/png", "blob": base64.StdEncoding.EncodeToString(tall.Bytes())}
	return resources
}

// frameFits is whether the host has sized the frame to what the view draws.
const frameFits = "Math.abs(f.iframe.clientHeight - d.getElementById('app').getBoundingClientRect().height) <= 1"

// goTo is the host showing the view in a display mode, and the view having
// followed.
func (h *host) goTo(i int, mode string) {
	h.t.Helper()
	h.eval(fmt.Sprintf("mmHost.setDisplayMode(%d, %q)", i, mode), nil)
	h.untilIn(i, fmt.Sprintf("d.documentElement.dataset.display === %q", mode))
}

func TestAViewDeclaresFullscreenAndFillsTheFrameItIsGiven(t *testing.T) {
	h := openHost(t)
	i := h.mount(showThread(map[string]any{"resources": tallImage(t)}))
	h.untilIn(i, "d.querySelector('.image-card img')")
	// A host offers its own way to fullscreen to a view that declares it.
	var declared []string
	h.in(i, "return f.bridge.getAppCapabilities().availableDisplayModes;", &declared)
	if !slices.Equal(declared, []string{"inline", "fullscreen"}) {
		t.Errorf("the view declares the display modes %v", declared)
	}

	// The thread goes as it is, and the view no longer sizes the frame.
	var posts int
	h.goTo(i, "fullscreen")
	h.in(i, "return d.querySelectorAll('.post').length;", &posts)
	if posts != 4 {
		t.Errorf("fullscreen draws %d of the thread's 4 posts", posts)
	}
	var sizes, after int
	h.in(i, "return f.sizes.length;", &sizes)
	h.in(i, `d.getElementById('app').append(d.createElement('p'));
		w.requestAnimationFrame(() => w.requestAnimationFrame(() => { d.body.dataset.drawn = 'true'; }));
		return null;`, nil)
	h.untilIn(i, "d.body.dataset.drawn === 'true'")
	h.in(i, "return f.sizes.length;", &after)
	if after != sizes {
		t.Errorf("the view reported its size %d times in fullscreen", after-sizes)
	}

	// An image opened there is alone, whole, in the frame, in its own shape.
	h.click(i, ".image-card")
	h.untilIn(i, "d.querySelector('.lightbox-image img') && d.querySelector('.lightbox-image img').clientHeight > 0")
	var drawn struct {
		Posts, Width, Height, Bottom, Frame, Page int
	}
	h.in(i, `const img = d.querySelector('.lightbox-image img');
		return {
			Posts: d.querySelectorAll('.post').length,
			Width: img.clientWidth,
			Height: img.clientHeight,
			Bottom: Math.ceil(img.getBoundingClientRect().bottom),
			Frame: w.innerHeight,
			Page: d.documentElement.scrollHeight,
		};`, &drawn)
	if drawn.Posts != 0 || drawn.Frame != 600 || drawn.Bottom > drawn.Frame || drawn.Page > drawn.Frame {
		t.Errorf("fullscreen draws the image as %+v", drawn)
	}
	if drawn.Height < drawn.Width*3-3 || drawn.Height > drawn.Width*3+3 {
		t.Errorf("a 1000 by 3000 image is fullscreen %d by %d", drawn.Width, drawn.Height)
	}

	// Closing it shows the thread again; the mode stays the host's.
	h.click(i, ".lightbox .close")
	h.untilIn(i, "!d.querySelector('.lightbox') && d.querySelectorAll('.post').length === 4")
	var mode string
	h.in(i, "return f.context.displayMode;", &mode)
	if mode != "fullscreen" {
		t.Errorf("closing the image left the view %s", mode)
	}
}

// An image open when the host changes the mode stays open, with the focus Escape
// needs, and the view sizes the frame again once it is back inline.
func TestAnOpenImageOutlastsTheHostChangingTheMode(t *testing.T) {
	h := openHost(t)
	i := h.mount(showThread(map[string]any{"resources": tallImage(t)}))
	h.untilIn(i, "d.querySelector('.image-card img')")
	h.click(i, ".image-card")
	h.untilIn(i, "d.querySelector('[data-post=\"p2\"] .lightbox-image img')")

	h.goTo(i, "fullscreen")
	h.untilIn(i, "d.querySelector('.lightbox-image img') && d.querySelectorAll('.post').length === 0")
	h.untilIn(i, "d.activeElement === d.querySelector('.lightbox .close')")

	h.goTo(i, "inline")
	h.untilIn(i, "d.querySelector('[data-post=\"p2\"] .lightbox-image img') && d.querySelectorAll('.post').length === 4")
	h.untilIn(i, "d.activeElement === d.querySelector('.lightbox .close')")
	h.untilIn(i, frameFits)

	// Closed in fullscreen, the view back inline is shorter than it left.
	h.goTo(i, "fullscreen")
	h.click(i, ".lightbox .close")
	h.untilIn(i, "!d.querySelector('.lightbox')")
	h.goTo(i, "inline")
	h.untilIn(i, frameFits)
}

// asked is what the view has asked the host for, in order.
func (h *host) asked(i int) []string {
	h.t.Helper()
	var asked []string
	h.in(i, "return f.asked;", &asked)
	return asked
}

// openImage waits for an image's card, and opens it.
func (h *host) openImage(i int) {
	h.t.Helper()
	h.untilIn(i, "d.querySelector('.image-card img')")
	h.click(i, ".image-card")
}

// An opened image is given the room the host has, alone and whole in the frame,
// and closing it takes the view back to the thread where it was.
func TestAnOpenedImageGoesFullscreenAndClosingItGoesBack(t *testing.T) {
	h := openHost(t)
	i := h.mount(showThread(map[string]any{"resources": tallImage(t), "grantDisplayModes": true}))
	h.openImage(i)
	h.untilIn(i, "d.documentElement.dataset.display === 'fullscreen'")
	h.untilIn(i, "d.querySelector('.lightbox-image img') && d.querySelector('.lightbox-image img').clientHeight > 0 && d.querySelectorAll('.post').length === 0")
	h.untilIn(i, "d.activeElement === d.querySelector('.lightbox .close')")
	var fits bool
	h.in(i, "return d.querySelector('.lightbox-image img').getBoundingClientRect().bottom <= w.innerHeight && d.documentElement.scrollHeight <= w.innerHeight;", &fits)
	if !fits {
		t.Error("the opened image does not fit the frame it was given")
	}

	h.click(i, ".lightbox .close")
	h.untilIn(i, "!d.querySelector('.lightbox') && d.documentElement.dataset.display === 'inline' && d.querySelectorAll('.post').length === 4")
	h.untilIn(i, frameFits)
	if got := h.asked(i); !slices.Equal(got, []string{"fullscreen", "inline"}) {
		t.Errorf("the view asked for %v", got)
	}

	// Opened again, it asks again: a yes stands.
	h.click(i, ".image-card")
	h.untilIn(i, "d.documentElement.dataset.display === 'fullscreen'")
	if got := h.asked(i); !slices.Equal(got, []string{"fullscreen", "inline", "fullscreen"}) {
		t.Errorf("the view asked for %v", got)
	}
}

// The request goes out in the person's click, before anything is waited for,
// since a host may hold it to one.
func TestAnOpenedImageAsksInTheClickItself(t *testing.T) {
	h := openHost(t)
	i := h.mount(showThread(map[string]any{"resources": tallImage(t)}))
	h.untilIn(i, "d.querySelector('.image-card img')")
	var sent []string
	h.in(i, `const seen = [];
		const real = window.postMessage;
		window.postMessage = function (message, target) { seen.push(String(message && message.method)); return real.call(window, message, target); };
		d.querySelector('.image-card').click();
		window.postMessage = real;
		return seen;`, &sent)
	if !slices.Contains(sent, "ui/request-display-mode") {
		t.Errorf("in the click itself, the view sent %v", sent)
	}
}

// A host that says no leaves the image open in its post, and is not asked again.
func TestAnImageARefusedHostLeavesOpenInItsPost(t *testing.T) {
	h := openHost(t)
	i := h.mount(showThread(map[string]any{"resources": tallImage(t)}))
	h.openImage(i)
	h.untilIn(i, "d.querySelector('[data-post=\"p2\"] .lightbox-image img')")
	h.untilIn(i, "f.asked.length === 1")
	var mode string
	h.in(i, "return d.documentElement.dataset.display;", &mode)
	if mode != "inline" {
		t.Errorf("after a no, the view is %s", mode)
	}
	h.untilIn(i, frameFits)
	h.click(i, ".lightbox .close")
	h.untilIn(i, "!d.querySelector('.lightbox')")
	h.openImage(i)
	h.untilIn(i, "d.querySelector('[data-post=\"p2\"] .lightbox-image img')")
	if got := h.asked(i); !slices.Equal(got, []string{"fullscreen"}) {
		t.Errorf("after a no, the view asked for %v", got)
	}
}

// A host that does not answer leaves the image open too, and is not asked again
// while it might.
func TestAnImageAHostDoesNotAnswerStaysOpenInItsPost(t *testing.T) {
	h := openHost(t)
	i := h.mount(showThread(map[string]any{"resources": tallImage(t), "holdDisplayModes": true}))
	h.openImage(i)
	h.untilIn(i, "d.querySelector('[data-post=\"p2\"] .lightbox-image img')")
	h.click(i, ".lightbox .close")
	h.untilIn(i, "!d.querySelector('.lightbox')")
	h.openImage(i)
	h.untilIn(i, "d.querySelector('[data-post=\"p2\"] .lightbox-image img')")
	if got := h.asked(i); !slices.Equal(got, []string{"fullscreen"}) {
		t.Errorf("with no answer, the view asked for %v", got)
	}
}

// A host that lists inline alone, as VS Code does, is never asked.
func TestAHostWithoutFullscreenIsNotAsked(t *testing.T) {
	h := openHost(t)
	i := h.mount(showThread(map[string]any{"resources": tallImage(t), "modes": []string{"inline"}}))
	h.openImage(i)
	h.untilIn(i, "d.querySelector('[data-post=\"p2\"] .lightbox-image img')")
	h.click(i, ".lightbox .close")
	h.untilIn(i, "!d.querySelector('.lightbox')")
	if got := h.asked(i); len(got) != 0 {
		t.Errorf("a host without fullscreen was asked for %v", got)
	}
}

// Already fullscreen by the host's own button, nothing is asked, and closing the
// image leaves the view where the host put it.
func TestAnImageInTheHostsOwnFullscreenAsksNothing(t *testing.T) {
	h := openHost(t)
	i := h.mount(showThread(map[string]any{"resources": tallImage(t), "grantDisplayModes": true}))
	h.untilIn(i, "d.querySelector('.image-card img')")
	h.goTo(i, "fullscreen")
	h.openImage(i)
	h.untilIn(i, "d.querySelector('.lightbox-image img')")
	h.click(i, ".lightbox .close")
	h.untilIn(i, "!d.querySelector('.lightbox') && d.querySelectorAll('.post').length === 4")
	if got := h.asked(i); len(got) != 0 {
		t.Errorf("in the host's own fullscreen, the view asked for %v", got)
	}
	var mode string
	h.in(i, "return f.context.displayMode;", &mode)
	if mode != "fullscreen" {
		t.Errorf("closing the image left the view %s", mode)
	}
}

// Taken back by the host's own way out, an image that went fullscreen on its own
// opening closes with it, and the view is the thread it was, with nothing to ask.
func TestAnImageThatWentFullscreenClosesWhenTheHostTakesItBack(t *testing.T) {
	h := openHost(t)
	i := h.mount(showThread(map[string]any{"resources": tallImage(t), "grantDisplayModes": true}))
	h.openImage(i)
	h.untilIn(i, "d.documentElement.dataset.display === 'fullscreen'")
	h.goTo(i, "inline")
	h.untilIn(i, "!d.querySelector('.lightbox') && d.querySelectorAll('.post').length === 4 && d.querySelector('[data-post=\"p2\"] .image-card')")
	h.untilIn(i, frameFits)
	if got := h.asked(i); !slices.Equal(got, []string{"fullscreen"}) {
		t.Errorf("the view asked for %v", got)
	}
	// And it opens as it did, again and again.
	h.openImage(i)
	h.untilIn(i, "d.documentElement.dataset.display === 'fullscreen' && d.querySelector('.lightbox-image img')")
	h.goTo(i, "inline")
	h.untilIn(i, "!d.querySelector('.lightbox')")
}

// Closed while the host is answering, there is nothing left to enlarge, so the
// view goes back.
func TestAnImageClosedBeforeTheAnswerGoesBack(t *testing.T) {
	h := openHost(t)
	i := h.mount(showThread(map[string]any{"resources": tallImage(t), "grantDisplayModes": true, "displayModeDelay": 300}))
	h.openImage(i)
	h.click(i, ".lightbox .close")
	h.untilIn(i, "!d.querySelector('.lightbox')")
	h.untilIn(i, "f.asked.length === 2 && f.context.displayMode === 'inline' && d.documentElement.dataset.display === 'inline' && d.querySelectorAll('.post').length === 4")
	if got := h.asked(i); !slices.Equal(got, []string{"fullscreen", "inline"}) {
		t.Errorf("the view asked for %v", got)
	}
}

// An image opened again after fullscreen opens as it did the first time, and
// neither leaves a scrollbar: one that appears while the host catches up narrows
// the view, and stays, since the narrowed view then fits.
func TestAnImageOpensTheSameAfterFullscreen(t *testing.T) {
	h := openHost(t)
	i := h.mount(showThread(map[string]any{"resources": tallImage(t), "width": 732}))
	h.untilIn(i, "d.querySelector('.image-card img')")
	type size struct{ Width, Height, Frame, Scrollbar int }
	measure := `const img = d.querySelector('.lightbox-image img'); return { Width: img.clientWidth, Height: img.clientHeight, Frame: f.iframe.clientHeight, Scrollbar: w.innerWidth - d.documentElement.clientWidth };`
	open := func() size {
		h.t.Helper()
		h.click(i, ".image-card")
		h.untilIn(i, "d.querySelector('.lightbox-image img') && d.querySelector('.lightbox-image img').clientHeight > 0")
		h.untilIn(i, frameFits)
		var got size
		h.in(i, measure, &got)
		return got
	}
	first := open()
	h.goTo(i, "fullscreen")
	h.untilIn(i, "d.querySelectorAll('.post').length === 0")
	h.goTo(i, "inline")
	h.untilIn(i, "d.querySelectorAll('.post').length === 4")
	h.untilIn(i, frameFits)
	h.click(i, ".lightbox .close")
	h.untilIn(i, "!d.querySelector('.lightbox')")
	h.untilIn(i, frameFits)
	second := open()
	if first != second || first.Scrollbar != 0 {
		t.Errorf("the image opened %+v the first time and %+v after fullscreen", first, second)
	}
}

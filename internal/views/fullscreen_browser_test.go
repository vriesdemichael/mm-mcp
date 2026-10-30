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

// Fullscreen is the host's, by a button of its own beside the view (ADR-022):
// the view declares that it can, and fills the frame it is given.

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

// A view has no way of its own to ask: nothing in it sends the request.
func TestAViewNeverAsksForADisplayMode(t *testing.T) {
	h := openHost(t)
	i := h.mount(showThread(map[string]any{"resources": tallImage(t)}))
	h.in(i, `f.asked = 0;
		f.bridge.onrequestdisplaymode = async () => { f.asked++; return { mode: 'inline' }; };
		return null;`, nil)
	h.untilIn(i, "d.querySelector('.image-card img')")
	h.click(i, ".image-card")
	h.untilIn(i, "d.querySelector('.lightbox-image img')")
	h.click(i, ".lightbox .close")
	h.untilIn(i, "!d.querySelector('.lightbox')")
	var asked int
	h.in(i, "return f.asked;", &asked)
	if asked != 0 {
		t.Errorf("the view asked for a display mode %d times", asked)
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

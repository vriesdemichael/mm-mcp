package server

import "testing"

// CSS names 148 colors (CSS Color 4, without transparent and currentcolor): a
// name missing from the list would drop an integration's bar.
func TestEveryColorCSSNamesIsKnown(t *testing.T) {
	t.Parallel()
	if len(cssNamedColors) != 148 {
		t.Errorf("%d named colors; CSS names 148", len(cssNamedColors))
	}
}

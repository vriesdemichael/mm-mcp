package server

import (
	"regexp"
	"strings"
)

// An integration chooses the color of the bar beside a card or a container.
// Mattermost 11.7 draws an attachment's bar in a hex color, or for good,
// warning and danger in its own (message_attachment.tsx). A server that draws
// blocks (MmBlocksEnabled) draws attachments as blocks too: it names those
// three, and default and primary, and hands any other color to the browser as
// --mm-blocks-accent-color (layout_blocks.tsx), so red or rgb(220, 0, 0)
// draws a bar there. A post keeps the colors the server draws, as the
// integration wrote them.

// hexColor is a color written in hex, the one way both releases take.
var hexColor = regexp.MustCompile(`^#([0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

// cssColorFunction is a color written as a CSS function, such as
// rgb(220, 0, 0) or hsl(0 80% 45% / 0.9): numbers with their units, separated
// by commas, spaces or a slash.
var cssColorFunction = regexp.MustCompile(`^(?i:rgba?|hsla?|hwb|lab|lch|oklab|oklch)\(\s*[-+]?[0-9.]+(?i:deg|grad|rad|turn|%)?((\s*[,/]\s*|\s+)[-+]?[0-9.]+(?i:deg|grad|rad|turn|%)?){2,3}\s*\)$`)

// cssNamedColors are the colors CSS names (CSS Color 4). transparent and
// currentcolor are left out: one draws nothing, the other the text's color.
var cssNamedColors = map[string]bool{}

func init() {
	for _, name := range strings.Fields(`aliceblue antiquewhite aqua aquamarine azure beige bisque black blanchedalmond blue
		blueviolet brown burlywood cadetblue chartreuse chocolate coral cornflowerblue cornsilk crimson cyan darkblue darkcyan
		darkgoldenrod darkgray darkgreen darkgrey darkkhaki darkmagenta darkolivegreen darkorange darkorchid darkred darksalmon
		darkseagreen darkslateblue darkslategray darkslategrey darkturquoise darkviolet deeppink deepskyblue dimgray dimgrey
		dodgerblue firebrick floralwhite forestgreen fuchsia gainsboro ghostwhite gold goldenrod gray green greenyellow grey
		honeydew hotpink indianred indigo ivory khaki lavender lavenderblush lawngreen lemonchiffon lightblue lightcoral
		lightcyan lightgoldenrodyellow lightgray lightgreen lightgrey lightpink lightsalmon lightseagreen lightskyblue
		lightslategray lightslategrey lightsteelblue lightyellow lime limegreen linen magenta maroon mediumaquamarine mediumblue
		mediumorchid mediumpurple mediumseagreen mediumslateblue mediumspringgreen mediumturquoise mediumvioletred midnightblue
		mintcream mistyrose moccasin navajowhite navy oldlace olive olivedrab orange orangered orchid palegoldenrod palegreen
		paleturquoise palevioletred papayawhip peachpuff peru pink plum powderblue purple rebeccapurple red rosybrown royalblue
		saddlebrown salmon sandybrown seagreen seashell sienna silver skyblue slateblue slategray slategrey snow springgreen
		steelblue tan teal thistle tomato turquoise violet wheat white whitesmoke yellow yellowgreen`) {
		cssNamedColors[name] = true
	}
}

// cssColor says whether a color is written as CSS writes one: in hex, by its
// CSS name, or as a CSS function of plain numbers.
func cssColor(color string) bool {
	return hexColor.MatchString(color) || cssNamedColors[strings.ToLower(color)] || cssColorFunction.MatchString(color)
}

// attachmentColor is the color an attachment's bar is drawn in, as the
// integration wrote it, or "" where Mattermost draws no bar for it. blocks
// says whether the server draws blocks (MmBlocksEnabled).
func attachmentColor(color string, blocks bool) string {
	if !blocks {
		if hexColor.MatchString(color) || color == "good" || color == "warning" || color == "danger" {
			return color
		}
		return ""
	}
	return containerAccent(strings.TrimSpace(color))
}

// containerAccent is the color a container's bar is drawn in, where
// Mattermost draws blocks: one it names, or a CSS color.
func containerAccent(color string) string {
	switch {
	case color == "default" || color == "primary" || color == "good" || color == "warning" || color == "danger":
		return color
	case cssColor(color):
		return color
	}
	return ""
}

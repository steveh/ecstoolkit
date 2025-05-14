package sessionutil

import (
	"fmt"
	"io"
	"os"

	"github.com/steveh/ecstoolkit/log"
	"github.com/steveh/ecstoolkit/message"
)

// DisplayMode represents a display mode for Unix-like systems.
type DisplayMode struct {
	logger log.T
}

// NewDisplayMode creates and initializes a new DisplayMode instance.
func NewDisplayMode(logger log.T) DisplayMode {
	displayMode := DisplayMode{
		logger: logger.With("subsystem", "DisplayMode"),
	}

	return displayMode
}

// DisplayMessage function displays the output on the screen.
func (d *DisplayMode) DisplayMessage(message message.ClientMessage) {
	var out io.Writer = os.Stdout

	if _, err := fmt.Fprint(out, string(message.Payload)); err != nil {
		d.logger.Error("Error writing message to output", "error", err)
	}
}

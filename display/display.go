// Package display implements an interface to HUB75 RGB LED matrix panels.
package display

import (
	"image/color"
	"machine"
	"strconv"
	"time"

	"tinygo.org/x/drivers/rgb75"
	"tinygo.org/x/tinyfont"

	"github.com/ardnew/weatherhub/model"
)

// Default constants for Display configuration.

// Default constants for Display configuration.
const (
	DefaultWidth      = 64 // px
	DefaultHeight     = 32 // px
	DefaultColorDepth = 4  // bits
)

// Display wraps the HUB75 device driver.
type Display struct {
	hub *rgb75.Device
	now *timeStamp
}

type timeStamp time.Time

// New returns a new Display initialized with given configuration.
// This method will always return a nil Display or a nil error. It will never
// return nil or non-nil for both Display and error.
func New(config rgb75.Config) (*Display, error) {

	// initialize the HUB75 device driver
	hub := rgb75.New(
		machine.HUB75_OE, machine.HUB75_LAT, machine.HUB75_CLK,
		[6]machine.Pin{
			machine.HUB75_R1, machine.HUB75_G1, machine.HUB75_B1,
			machine.HUB75_R2, machine.HUB75_G2, machine.HUB75_B2,
		},
		[]machine.Pin{
			machine.HUB75_ADDR_A, machine.HUB75_ADDR_B, machine.HUB75_ADDR_C,
			machine.HUB75_ADDR_D, machine.HUB75_ADDR_E,
		})

	// configure the display
	if 0 == config.Width {
		config.Width = DefaultWidth
	}
	if 0 == config.Height {
		config.Height = DefaultHeight
	}
	if 0 == config.ColorDepth {
		config.ColorDepth = DefaultColorDepth
	}
	if err := hub.Configure(config); nil != err {
		return nil, err
	}

	// initialize and begin updating screen
	hub.ClearDisplay()
	hub.Resume()

	return &Display{hub: hub, now: &timeStamp{}}, nil
}

func (d *Display) Update(data model.Model) {
	// Update is only called if the Model data has changed. When the model data
	// changes, we redraw the entire display so that we don't leave stale pixels
	// in the background.
	// This could be improved to only redraw the regions that need updating, but
	// the redrawing occurs quite quickly with this much-simpler technique.

	width, height := d.hub.Size()

	switch data.Status {
	case model.StatusIdle, model.StatusDisconnected:
		d.hub.ClearDisplay()
		tinyfont.WriteLine(d.hub, &tinyfont.TomThumb, 0, height-2, "Disconnected",
			color.RGBA{R: 0xFF, G: 0x00, B: 0x00, A: 0xFF})

	case model.StatusConnecting:
		d.hub.ClearDisplay()
		tinyfont.WriteLine(d.hub, &tinyfont.TomThumb, 0, height-2, "Connecting...",
			color.RGBA{R: 0x00, G: 0x00, B: 0xFF, A: 0xFF})

	case model.StatusUnsynchronized:
		d.hub.ClearDisplay()
		str := "Synchronizing"
		if data.Retry > 0 {
			str += "(" + strconv.FormatUint(uint64(data.Retry), 10) + ")"
		}
		str += "..."
		tinyfont.WriteLine(d.hub, &tinyfont.TomThumb, 0, height-2, str,
			color.RGBA{R: 0x00, G: 0xFF, B: 0x00, A: 0xFF})

	case model.StatusSynchronized:

		const rowHeight = 6

		new, dow, doy, tim := d.now.set(data.Time)
		if new {
			d.hub.ClearDisplay()
		}

		if "" != tim {
			var (
				timeWidth      int16 = 4*6 + 3*2
				tx, ty         int16 = width - timeWidth, 2 + rowHeight
				px, py, pw, ph int16 = width - timeWidth, 2, timeWidth, rowHeight
			)
			d.fillRect(px, py, pw, ph, color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x00})
			tinyfont.WriteLine(d.hub, &tinyfont.TomThumb, tx, ty, tim,
				color.RGBA{R: 0x00, G: 0xFF, B: 0x00, A: 0xFF})
		}
		if "" != dow {
			var (
				tx, ty         int16 = 0, height - 1*rowHeight - 2
				px, py, pw, ph int16 = 0, height - 2*rowHeight - 2, 64, rowHeight
			)
			d.fillRect(px, py, pw, ph, color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x00})
			tinyfont.WriteLine(d.hub, &tinyfont.TomThumb, tx, ty, dow,
				color.RGBA{R: 0x00, G: 0xFF, B: 0xFF, A: 0xFF})
		}
		if "" != doy {
			var (
				tx, ty         int16 = 0, height - 0*rowHeight - 2
				px, py, pw, ph int16 = 0, height - 1*rowHeight - 2, 64, rowHeight
			)
			d.fillRect(px, py, pw, ph, color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x00})
			tinyfont.WriteLine(d.hub, &tinyfont.TomThumb, tx, ty, doy,
				color.RGBA{R: 0x00, G: 0x00, B: 0xFF, A: 0xFF})
		}
	}
}

func (d *Display) clipRect(x, y, w, h int16) (bool, int16, int16, int16, int16) {
	// normalize width/height to be positive
	if w < 0 {
		x, w = x+w, -w // adjust x by w, change sign of w
	}
	if h < 0 {
		y, h = y+h, -h // adjust y by h, change sign of h
	}
	// ensure origin is within bounds
	sx, sy := d.hub.Size()
	if x < 0 {
		x, w = 0, w+x // clip x to origin, adjust w by x
	} else if x >= sx {
		return false, 0, 0, 0, 0 // beyond screen bounds
	}
	if y < 0 {
		y, h = 0, h+y // clip y to origin, adjust h by y
	} else if y >= sy {
		return false, 0, 0, 0, 0 // beyond screen bounds
	}
	// ensure rect bounds is within screen bounds
	if x+w >= sx {
		w = sx - x // clip w to screen width
	}
	if y+h >= sy {
		h = sy - y // clip h to screen height
	}
	return true, x, y, w, h
}

func (d *Display) fillRect(x, y, w, h int16, c color.RGBA) {
	var ok bool
	if ok, x, y, w, h = d.clipRect(x, y, w, h); ok {
		for row := y; row < y+h; row++ {
			for col := x; col < x+w; col++ {
				d.hub.SetPixel(col, row, c)
			}
		}
	}
}

func (s *timeStamp) set(t time.Time) (new bool, dow, doy, tim string) {
	p := time.Time(*s)
	new = p.IsZero()
	if new || p.Weekday() != t.Weekday() {
		dow = t.Weekday().String()
	}
	if new || p.YearDay() != t.YearDay() {
		doy = t.Format("January 2")
	}
	tim = t.Format("15:04:05")
	*s = timeStamp(t) // update the saved timestamp
	return
}

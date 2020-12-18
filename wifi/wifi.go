// Package wifi implements an interface to the WiFi coprocessor.
package wifi

import (
	"errors"
	"machine"
	"time"

	"tinygo.org/x/drivers/net"
	"tinygo.org/x/drivers/wifinina"

	"github.com/ardnew/weatherhub/model"
	"github.com/ardnew/weatherhub/wifi/network"
)

var (
	ErrConnectToAP  = errors.New("failed to connect to access point")
	ErrNoIPAddress  = errors.New("could not obtain IP address from access point")
	ErrNotConnected = errors.New("not connected to access point")
)

// WiFi wraps the WiFiNINA device driver.
type WiFi struct {
	nina *wifinina.Device
	ip   wifinina.IPAddress
}

// New returns a new WiFi using the default peripherals and GPIO pins.
// The SPI interface connected to the WiFi coprocessor is also initialized and
// configured for use.
// This method will always return a nil WiFi or a nil error. It will never
// return nil or non-nil for both WiFi and error.
func New() (*WiFi, error) {

	// configure the SPI interface connected to ESP32
	spiConfig := machine.SPIConfig{
		Frequency: 8 * 1.0E6,
		SDO:       machine.NINA_SDO,
		SDI:       machine.NINA_SDI,
		SCK:       machine.NINA_SCK,
	}
	machine.NINA_SPI.Configure(spiConfig)

	// configure the WiFiNINA driver
	nina := &wifinina.Device{
		SPI:   machine.NINA_SPI,
		CS:    machine.NINA_CS,
		ACK:   machine.NINA_ACK,
		GPIO0: machine.NINA_GPIO0,
		RESET: machine.NINA_RESETN,
	}
	nina.Configure()

	return &WiFi{nina: nina}, nil
}

// Connect establishes an AP connection using given SSID and passphrase.
// An error is returned if the AP could not be reached or an IP not obtained.
func (w *WiFi) Connect(ap network.AP) error {

	// attempt to connect to SSID with passphrase
	time.Sleep(2 * time.Second)
	w.nina.SetPassphrase(ap.SSID, ap.Pass)

	// wait for connection established
	if !w.waitWithTimeout(w.isConnected) {
		return ErrConnectToAP
	}
	// wait for DHCP IP lease
	if !w.waitWithTimeout(w.hasIP) {
		return ErrNoIPAddress
	}

	// update model with our connection details
	model.Set(func(m *model.Model) {
		m.AP, m.IP = ap, w.ip
	})

	return nil
}

func (w *WiFi) GetHostByName(name string) (net.IP, error) {
	if !w.isConnected() || !w.hasIP() {
		return nil, ErrNotConnected
	}
	addr, err := w.nina.GetHostByName(name)
	if nil != err {
		return nil, err
	}
	return net.ParseIP(addr.String()), nil
}

func (w *WiFi) waitWithTimeout(ready func() bool) (ok bool) {
	const (
		maxAttempts = 8
		baseTimeout = 125 * time.Millisecond
	)
	attempt, timeout := 0, baseTimeout
	for !ok && attempt < maxAttempts {
		if ok = ready(); !ok {
			time.Sleep(timeout)
			timeout <<= 1
		}
	}
	return
}

func (w *WiFi) isConnected() bool {
	stat, _ := w.nina.GetConnectionStatus()
	return wifinina.StatusConnected == stat
}

func (w *WiFi) hasIP() bool {
	var err error
	w.ip, _, _, err = w.nina.GetIP()
	return nil == err
}

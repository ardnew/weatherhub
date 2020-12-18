package main

import (
	"errors"
	"time"

	"tinygo.org/x/drivers/rgb75"

	"github.com/ardnew/weatherhub/display"
	"github.com/ardnew/weatherhub/run"
	"github.com/ardnew/weatherhub/wifi"
	"github.com/ardnew/weatherhub/wifi/ntp"
)

var (
	ErrNotConnected = errors.New("could not connect to any preferred access point")
)

func main() {
	// initialize the HUB75 display
	disp, err := display.New(rgb75.Config{})
	if nil != err {
		halt(err)
	}
	// initialize the network interface
	net, err := wifi.New()
	if nil != err {
		halt(err)
	}
	// initialize the NTP client
	host := ntp.New(net, ntp.Config{})
	// enter state machine
	run.Run(disp, net, host)
}

func halt(err error) {
	for {
		println("error: " + err.Error())
		time.Sleep(time.Second)
	}
}

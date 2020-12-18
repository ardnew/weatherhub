package run

import (
	"time"

	"github.com/ardnew/weatherhub/display"
	"github.com/ardnew/weatherhub/model"
	"github.com/ardnew/weatherhub/wifi"
	"github.com/ardnew/weatherhub/wifi/network"
	"github.com/ardnew/weatherhub/wifi/ntp"
)

func Run(disp *display.Display, net *wifi.WiFi, host *ntp.NTP) {

	// initial state
	model.Set(func(m *model.Model) {
		m.Status = model.StatusDisconnected
	})

	// main run loop
	for {
		if changed, data := model.Get(); changed {

			// something in the Model has changed. update the display with current
			// Model data, and then perform any transition logic.

			disp.Update(data)
			switch data.Status {
			case model.StatusIdle, model.StatusDisconnected:
				// transition to initiate connection
				model.Set(func(m *model.Model) {
					m.Status = model.StatusConnecting
				})

			case model.StatusConnecting:
				// try to connect to each known AP, in order
				for _, ap := range network.Network {
					if err := net.Connect(ap); nil != err {
						println(ap.SSID + ": " + err.Error())
					} else {
						// no error, we successfully connected
						model.Set(func(m *model.Model) {
							m.Status = model.StatusUnsynchronized
						})
					}
				}

			case model.StatusUnsynchronized:
				// try to synchronize system time with NTP server
				model.Mod(func(m *model.Model) { m.Retry = 0 })
				if err := host.Sync(); nil != err {
					println("error: " + err.Error())
				} else {
					// no error, transition to synchronized state
					model.Set(func(m *model.Model) {
						m.Status = model.StatusSynchronized
					})
				}

			case model.StatusSynchronized:
				// synchronize Model time with current system time.
				if err := host.Sync(); nil != err {
					println("error: " + err.Error())
					// caught an error, transition back to unsynchronized state
					model.Set(func(m *model.Model) {
						m.Status = model.StatusUnsynchronized
					})
				}
			}

		} else {

			// nothing has changed, we are continuing on in the same state as the
			// previous iteration. perform any idle or maintenance logic.
			// do NOT update the display.

			switch data.Status {
			case model.StatusUnsynchronized:
				// retry to synchronize system time with NTP server
				model.Mod(func(m *model.Model) { m.Retry++ })
				if err := host.Sync(); nil != err {
					println("error: " + err.Error())
				} else {
					// no error, transition to synchronized state
					model.Set(func(m *model.Model) {
						m.Status = model.StatusSynchronized
					})
				}

			case model.StatusSynchronized:
				// synchronize Model time with current system time.
				if err := host.Sync(); nil != err {
					println("error: " + err.Error())
					// caught an error, transition back to unsynchronized state
					model.Set(func(m *model.Model) {
						m.Status = model.StatusUnsynchronized
					})
				}
			}
		}

		time.Sleep(10 * time.Millisecond)
	}
}

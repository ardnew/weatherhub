package ntp

import (
	"errors"
	// "fmt"
	"runtime"
	"time"

	"tinygo.org/x/drivers/net"

	"github.com/ardnew/weatherhub/model"
	"github.com/ardnew/weatherhub/wifi"
)

var DefaultServer = []string{"us.pool.ntp.org", "time.google.com"}

const (
	DefaultRemotePort = 123
	DefaultLocalPort  = 2390
	DefaultTZOffset   = -6 * 60 * 60 // CST(-6)
	DefaultInterval   = 6 * time.Hour
	DefaultPrecision  = time.Second
	DefaultLeapSmear  = false // ** only if using Google NTP (time.google.com) **
)

var (
	ErrReadDatagramSize = errors.New("received unexpected NTP datagram size")
	ErrReadNoResponse   = errors.New("timeout waiting for NTP datagram reply")
)

type Config struct {
	Server     []string
	RemotePort int
	LocalPort  int
	TZOffset   int
	Interval   time.Duration // how often to synchronize with NTP server
	Precision  time.Duration // how often to update Model with synchronized time
	LeapSmear  bool          // https://developers.google.com/time/faq#libit
}

type NTP struct {
	device   *wifi.WiFi
	config   Config
	locale   *time.Location
	lastSync time.Time
	lastPost time.Time
	datagram datagram
}

const datagramSize = 48

type datagram []uint8

func New(device *wifi.WiFi, config Config) *NTP {

	if config.Server == nil || len(config.Server) == 0 {
		config.Server = DefaultServer
		config.LeapSmear = DefaultLeapSmear
	}
	if config.RemotePort == 0 {
		config.RemotePort = DefaultRemotePort
	}
	if config.LocalPort == 0 {
		config.LocalPort = DefaultLocalPort
	}
	if config.TZOffset == 0 {
		config.TZOffset = DefaultTZOffset
	}
	if config.Interval == 0 {
		config.Interval = DefaultInterval
	}
	if config.Precision == 0 {
		config.Precision = DefaultPrecision
	}

	return &NTP{
		device:   device,
		config:   config,
		locale:   time.FixedZone("localtime", config.TZOffset),
		datagram: make(datagram, datagramSize),
	}
}

func (n *NTP) Sync() error {

	// check if we need to re-sync with the NTP server and/or update the Model
	systemExpired, modelExpired := n.isExpired(time.Now())

	// synchronization with NTP server should occur very infrequently, which will
	// save bandwidth, power, and help alleviate intermittent connectivity.
	// once synchronized, we can rely on the internal low-power RTC to keep time.
	if systemExpired {
		// construct UDP end points
		_, m := model.Get()
		idx := m.Retry % uint(len(n.config.Server))
		host, err := n.device.GetHostByName(n.config.Server[idx])
		if nil != err {
			return err
		}
		radd := &net.UDPAddr{IP: host, Port: n.config.RemotePort}
		ladd := &net.UDPAddr{Port: n.config.LocalPort}
		// create UDP socket
		conn, err := net.DialUDP("udp", ladd, radd)
		if nil != err {
			return err
		}
		// send NTP request
		curr, err := n.request(conn)
		// curr, err := getCurrentTime(conn)
		if nil != err {
			return err
		}
		// close the socket
		conn.Close()
		// update system time
		runtime.AdjustTimeOffset(-1 * int64(time.Since(curr)))
		n.lastSync = time.Now()
	}

	// all other packages in the program rely on the Model data as time keeper.
	// update it as often as requested by Config field Precision.
	if modelExpired {
		n.lastPost = time.Now()
		model.Set(func(m *model.Model) {
			m.Time = n.lastPost.In(n.locale)
		})
	}

	return nil
}

func isExpired(at, since time.Time, span time.Duration) bool {
	return at.IsZero() || at.Sub(since) >= span
}

func (n *NTP) isExpired(at time.Time) (system, model bool) {
	return isExpired(at, n.lastSync, n.config.Interval),
		isExpired(at, n.lastPost, n.config.Precision)
}

func (n *NTP) request(conn *net.UDPSerialConn) (time.Time, error) {
	if err := n.write(conn); nil != err {
		return time.Time{}, err
	}
	if err := n.read(conn); nil != err {
		return time.Time{}, err
	}
	return n.datagram.parse(), nil
}

func (n *NTP) write(conn *net.UDPSerialConn) error {
	// clear the datagram buffer
	n.datagram.reset()
	// populate datagram buffer with an NTP request
	n.datagram[0] = 0b11100011 // LI, Version, Mode
	if !n.config.LeapSmear {
		// set LI to alarm (clock not sync'd) if server does not leap smear:
		n.datagram[0] |= 0b00000011
	}
	n.datagram[1] = 0    // Stratum, or type of clock
	n.datagram[2] = 6    // Polling Interval
	n.datagram[3] = 0xEC // Peer Clock Precision
	// 8 bytes of zero for Root Delay & Root Dispersion
	n.datagram[12] = 49
	n.datagram[13] = 0x4E
	n.datagram[14] = 49
	n.datagram[15] = 52
	// write datagram to socket
	_, err := conn.Write(n.datagram)
	return err
}

func (n *NTP) read(conn *net.UDPSerialConn) error {
	// clear the datagram buffer
	n.datagram.reset()
	// keep reading the socket until we've received a reply
	const timeout = 2 * time.Second
	start := time.Now()
	for time.Since(start) <= timeout {
		time.Sleep(5 * time.Millisecond)
		// poll the socket
		if n, err := conn.Read(n.datagram); nil != err {
			return err
		} else if n == 0 {
			continue // no packet received yet, try again
		} else if n != datagramSize {
			return ErrReadDatagramSize
		}
		// read result passed all constraints, return valid reply
		return nil
	}
	return ErrReadNoResponse
}

func (d *datagram) reset() {
	for i := range *d {
		(*d)[i] = 0 // zeroize the buffer
	}
}

func (d *datagram) parse() time.Time {
	const seventyYears = 2208988800
	t := uint32((*d)[40])<<24 | uint32((*d)[41])<<16 |
		uint32((*d)[42])<<8 | uint32((*d)[43])
	return time.Unix(int64(t-seventyYears), 0)
}

// Package model implements a global singleton data structure with functions for
// synchronized read+write access.
package model

import (
	"sync"
	"time"

	"tinygo.org/x/drivers/wifinina"

	"github.com/ardnew/weatherhub/wifi/network"
)

// Model defines a singleton for global data shared by all consuming packages.
//
// The instance variable of this type is private, so consumers must use one of
// the package's exported functions to access or modify its content, which
// provide automatic synchronization.
type Model struct {
	AP     network.AP
	IP     wifinina.IPAddress
	Time   time.Time
	Retry  uint
	Status Status
}

// Status represents the current position of the program state machine.
type Status uint8

// Constants defining each possible program Status.
const (
	StatusIdle Status = iota
	StatusDisconnected
	StatusConnecting
	StatusUnsynchronized
	StatusSynchronized
)

// state holds the instance variable of singleton type Model and other fields
// used for access control.
// See godoc on type Model for details.
var state = struct {
	lock    *sync.Mutex
	data    Model
	changed bool
}{
	lock: &sync.Mutex{},
}

// Get safely returns the model's changed flag and a copy of the Model data (as
// it was defined when Get was called).
// The changed flag is automatically set false after reading.
func Get() (changed bool, data Model) {
	state.lock.Lock()
	changed, data = state.changed, state.data
	state.changed = false
	state.lock.Unlock()
	return
}

// Set provides synchronized read+write access to the Model data via argument to
// the given closure.
// The changed flag is automatically set true after the closure has been called.
func Set(set func(*Model)) {
	state.lock.Lock()
	set(&state.data)
	state.changed = true
	state.lock.Unlock()
}

// Mod provides synchronized read+write access to the Model data via argument to
// the given closure.
// The changed flag is unaffected by this method.
func Mod(mod func(*Model)) {
	state.lock.Lock()
	mod(&state.data)
	state.lock.Unlock()
}

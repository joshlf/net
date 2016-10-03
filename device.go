package net

// TODO(joshlf): Maybe rename Device to IPDevice
// These Devices only support IP operations, and
// maybe we want a lower-level device interface
// that allows writing directly to link-layer
// addresses. Alternatively:
//   - That interface should be called LinkDevice
//     (or similar)
//   - Those devices require special per-device
//     types (e.g., an ethernet MAC address), so
//     a common Go interface doesn't make sense.

// A Device is a handle on a physical or virtual network device. A Device
// must implement the IPv4Device or IPv6Device interfaces, although
// it may also implement both.
//
// Devices are safe for concurrent access.
type Device interface {
	// BringUp brings the Device up. If it is already up,
	// BringUp is a no-op.
	BringUp() error
	// BringDown brings the Device down. If it is already down,
	// BringDown is a no-op.
	BringDown() error
	// IsUp returns true if the Device is up.
	IsUp() bool

	// MTU returns the device's maximum transmission unit,
	// or 0 if no MTU is set.
	MTU() uint64

	// // Read reads a packet from the device,
	// // copying the payload into b. It returns the
	// // number of bytes copied into b and the return
	// // address and protocol that were on the packet.
	// // ReadFrom can be made to time out and return
	// // an error with Timeout() == true after a fixed
	// // time limit; see SetDeadline and SetReadDeadline.
	// //
	// // If a packet larger than len(b) is received,
	// // n will be len(b), and err will be io.EOF
	// Read(b []byte) (n int, err error)
	// // WriteTo writes a packet to the device with
	// // the specified destination address.
	// // WriteTo can be made to time out and return
	// // an error with Timeout() == true after a fixed time limit;
	// // see SetDeadline and SetWriteDeadline.
	// // On device connections, write timeouts are rare.
	// //
	// // If len(b) is larger than the device's MTU,
	// // WriteTo will not write the packet, and will
	// // return an MTU error (see IsMTU).
	// WriteTo(b []byte, dst IP) (n int, err error)
	Deadliner
}

// An IPv4Device is a Device with IPv4-specific methods.
type IPv4Device interface {
	Device

	// IPv4 returns the device's IPv4 address and network mask
	// if they have been set.
	IPv4() (ok bool, addr, netmask IPv4)
	// SetIPv4 sets the device's IPv4 address and network mask,
	// returning any error encountered. SetIPv4 can only be
	// called when the device is down.
	//
	// Calling SetIPv4 with the zero value for addr unsets
	// the IPv4 address.
	SetIPv4(addr, netmask IPv4) error

	// ReadIPv4 is like Device's Read,
	// but for IPv4 only.
	ReadIPv4(b []byte) (n int, err error)
	// WriteToIPv4 is like Device's WriteTo,
	// but for IPv4 only.
	WriteToIPv4(b []byte, dst IPv4) (n int, err error)
}

// An IPv6Device is a Device with IPv6-specific methods.
type IPv6Device interface {
	Device

	// IPv6 returns the device's IPv6 address and network mask
	// if they have been set.
	IPv6() (ok bool, addr, netmask IPv6)
	// SetIPv6 sets the device's IPv6 address and network mask,
	// returning any error encountered. SetIPv6 can only be
	// called when the device is down.
	//
	// Calling SetIPv6 with the zero value for addr unsets
	// the IPv6 address.
	SetIPv6(addr, netmask IPv6) error

	// ReadIPv6 is like Device's Read,
	// but for IPv6 only.
	ReadIPv6(b []byte) (n int, err error)
	// WriteToIPv6 is like Device's WriteTo,
	// but for IPv6 only.
	WriteToIPv6(b []byte, dst IPv6) (n int, err error)
}

//
// // A DeviceSet is a set of named Devices. A DeviceSet is safe for concurrent
// // access.
// type DeviceSet struct {
// 	devices []namedDevice
// 	mu      sync.RWMutex
// }
//
// type namedDevice struct {
// 	name string
// 	dev  Device
// }
//
// // Get gets the named Device.
// func (d *DeviceSet) Get(name string) (ok bool, dev Device) {
// 	d.mu.RLock()
// 	for _, d := range d.devices {
// 		if d.name == name {
// 			ok, dev = true, d.dev
// 		}
// 	}
// 	d.mu.RUnlock()
// 	return ok, dev
// }
//
// // GetName gets the name for the given Device.
// func (d *DeviceSet) GetName(dev Device) (ok bool, name string) {
// 	d.mu.RLock()
// 	for _, d := range d.devices {
// 		if d.dev == dev {
// 			ok, name = true, d.name
// 		}
// 	}
// 	d.mu.RUnlock()
// 	return ok, name
// }
//
// // ListNames returns a list of device names.
// func (d *DeviceSet) ListNames() (names []string) {
// 	d.mu.RLock()
// 	for _, d := range d.devices {
// 		names = append(names, d.name)
// 	}
// 	d.mu.RUnlock()
// 	return names
// }
//
// // Put stores a new Device under the given name, overwriting any previous
// // Device by the same name. If dev is nil, any Device with the given name
// // will be removed.
// func (d *DeviceSet) Put(name string, dev Device) {
// 	d.mu.Lock()
// 	defer d.mu.Unlock()
// 	for i, dd := range d.devices {
// 		if dd.name == name {
// 			if dev == nil {
// 				// remove
// 				copy(d.devices[i:], d.devices[i+1:])
// 				d.devices = d.devices[:len(d.devices)]
// 			} else {
// 				// overwrite
// 				d.devices[i].dev = dev
// 			}
// 			return
// 		}
// 	}
// 	d.devices = append(d.devices, namedDevice{name: name, dev: dev})
// }

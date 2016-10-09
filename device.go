package net

import "sync"

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
	MTU() int
}

// An IPv4Device is a Device with IPv4-specific methods.
type IPv4Device interface {
	Device

	// IPv4 returns the device's IPv4 address and network mask
	// if they have been set.
	IPv4() (addr, netmask IPv4, ok bool)
	// SetIPv4 sets the device's IPv4 address and network mask,
	// returning any error encountered. SetIPv4 can only be
	// called when the device is down.
	//
	// Calling SetIPv4 with the zero value for addr unsets
	// the IPv4 address.
	SetIPv4(addr, netmask IPv4) error

	// RegisterIPv4Callback registers f as the function
	// to be called when a new IPv4 packet arrives. It
	// overwrites any previously-registered callbacks.
	// If f is nil, incoming IPv4 packets will be dropped.
	RegisterIPv4Callback(f func([]byte))
	// WriteToIPv4 is like Device's WriteTo,
	// but for IPv4 only.
	WriteToIPv4(b []byte, dst IPv4) (n int, err error)
}

// An IPv6Device is a Device with IPv6-specific methods.
type IPv6Device interface {
	Device

	// IPv6 returns the device's IPv6 address and network mask
	// if they have been set.
	IPv6() (addr, netmask IPv6, ok bool)
	// SetIPv6 sets the device's IPv6 address and network mask,
	// returning any error encountered. SetIPv6 can only be
	// called when the device is down.
	//
	// Calling SetIPv6 with the zero value for addr unsets
	// the IPv6 address.
	SetIPv6(addr, netmask IPv6) error

	// RegisterIPv6Callback registers f as the function
	// to be called when a new IPv4 packet arrives. It
	// overwrites any previously-registered callbacks.
	// If f is nil, incoming IPv4 packets will be dropped.
	RegisterIPv6Callback(f func([]byte))
	// WriteToIPv6 is like Device's WriteTo,
	// but for IPv6 only.
	WriteToIPv6(b []byte, dst IPv6) (n int, err error)
}

// A DeviceSet is a set of named Devices. A DeviceSet is safe for concurrent
// access. The zero value DeviceSet is a valid DeviceSet.
type DeviceSet struct {
	// since the zero value is valid, byName and byDevice might be nil;
	// make sure to check first when modifying them
	byName   map[string]Device
	byDevice map[Device]string
	mu       sync.RWMutex
}

// Get gets the named Device.
func (d *DeviceSet) Get(name string) (dev Device, ok bool) {
	d.mu.RLock()
	dev, ok = d.byName[name]
	d.mu.RUnlock()
	return dev, ok
}

// GetName gets the name for the given Device.
func (d *DeviceSet) GetName(dev Device) (name string, ok bool) {
	d.mu.RLock()
	name, ok = d.byDevice[dev]
	d.mu.RUnlock()
	return name, ok
}

// ListNames returns a list of device names.
func (d *DeviceSet) ListNames() (names []string) {
	d.mu.RLock()
	for name := range d.byName {
		names = append(names, name)
	}
	d.mu.RUnlock()
	return names
}

// Put stores a new Device under the given name, overwriting any previous
// Device by the same name. If dev is nil, any Device with the given name
// will be removed.
func (d *DeviceSet) Put(name string, dev Device) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if dev == nil {
		if d.byName == nil {
			// the set is empty, so this delete operation is a no-op
			return
		}
		d.remove(name)
	} else {
		if d.byName == nil {
			d.byName = make(map[string]Device)
			d.byDevice = make(map[Device]string)
		}
		d.remove(name)
		d.byName[name] = dev
		d.byDevice[dev] = name
	}
}

// assumes d.mu.Lock, and that name is stored along with a corresponding device
func (d *DeviceSet) remove(name string) {
	dev := d.byName[name]
	delete(d.byName, name)
	delete(d.byDevice, dev)
}

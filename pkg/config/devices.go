package config

import "strings"

// FindDevice looks up a device by IEEE address or friendly name.
func (c *Config) FindDevice(id string) *DeviceEntry {
	for i := range c.Devices {
		if strings.EqualFold(c.Devices[i].IEEEAddress, id) || strings.EqualFold(c.Devices[i].FriendlyName, id) {
			return &c.Devices[i]
		}
	}
	return nil
}

// AddOrUpdateDevice inserts a new device or updates an existing one matched by IEEE address.
func (c *Config) AddOrUpdateDevice(d DeviceEntry) {
	for i := range c.Devices {
		if strings.EqualFold(c.Devices[i].IEEEAddress, d.IEEEAddress) {
			// Preserve friendly name if the incoming entry has none
			if d.FriendlyName == "" || d.FriendlyName == d.IEEEAddress {
				d.FriendlyName = c.Devices[i].FriendlyName
			}
			c.Devices[i] = d
			return
		}
	}
	c.Devices = append(c.Devices, d)
}

// RemoveDevice deletes a device by IEEE address.
func (c *Config) RemoveDevice(ieee string) {
	for i := range c.Devices {
		if strings.EqualFold(c.Devices[i].IEEEAddress, ieee) {
			c.Devices = append(c.Devices[:i], c.Devices[i+1:]...)
			return
		}
	}
}

// RenameDevice updates the friendly name of a device identified by IEEE address.
func (c *Config) RenameDevice(ieee, newName string) {
	for i := range c.Devices {
		if strings.EqualFold(c.Devices[i].IEEEAddress, ieee) {
			c.Devices[i].FriendlyName = newName
			return
		}
	}
}

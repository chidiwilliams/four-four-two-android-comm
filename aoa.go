package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/gousb"
)

type id int

func (id id) in(ns ...int) bool {
	for _, i := range ns {
		if int(id) == i {
			return true
		}
	}
	return false
}

type deviceIdentity struct {
	Bus     int
	Address int
	Vendor  gousb.ID
	Product gousb.ID
}

func readDeviceIdentity(d *gousb.DeviceDesc) deviceIdentity {
	return deviceIdentity{d.Bus, d.Address, d.Vendor, d.Product}
}

func (i deviceIdentity) nil() bool {
	return i.Bus == 0 && i.Address == 0 && i.Vendor == 0 && i.Product == 0
}

func (i deviceIdentity) match(d *gousb.DeviceDesc) bool {
	return i.Bus == d.Bus &&
			i.Address == d.Address &&
			i.Vendor == d.Vendor &&
			i.Product == d.Product
}

func (i deviceIdentity) isAccessoryMode() bool {
	return i.Vendor == 0x18D1 && id(i.Product).in(0x2D00, 0x2D01)
}

type deviceHistory int

const (
	historyNoAction = iota
	historySwitchRequested
	historySwitchFailed
	historyOpenFailed
)

type deviceMap map[deviceIdentity]deviceHistory

func mapDevices() deviceMap {
	ctx := gousb.NewContext()
	defer func() { _ = ctx.Close() }()

	m := make(deviceMap)
	_, _ = ctx.OpenDevices(func(d *gousb.DeviceDesc) bool {
		m[readDeviceIdentity(d)] = historyNoAction
		return false
	})

	return m
}

func propagateDeviceHistory(i deviceIdentity, h deviceHistory) deviceHistory {
	if i.isAccessoryMode() {
		if h == historyOpenFailed {
			return historyOpenFailed
		} else {
			return historyNoAction
		}
	} else {
		if h == historySwitchRequested {
			log.Printf("Not yet switched, treat as failed: %v", i)
			return historySwitchFailed
		} else {
			return h
		}
	}
}

func updateDeviceMap(new deviceMap, old deviceMap) (deviceMap, deviceIdentity, deviceIdentity) {
	var identityOfAccessoryMode, identityToSwitch deviceIdentity
	m := make(deviceMap)

	for identity, blank := range new {
		history, ok := old[identity]
		if ok {
			history = propagateDeviceHistory(identity, history)
		} else {
			history = blank
		}

		m[identity] = history

		if identity.isAccessoryMode() && history == historyNoAction {
			identityOfAccessoryMode = identity
		} else if !identity.isAccessoryMode() && history == historyNoAction {
			identityToSwitch = identity
		}
	}
	return m, identityOfAccessoryMode, identityToSwitch
}

func findConfig(d *gousb.DeviceDesc) (*gousb.ConfigDesc, error) {
	if len(d.Configs) <= 0 {
		return nil, fmt.Errorf("no config descriptor found")
	}

	var found = false
	var lowest int
	var cfg gousb.ConfigDesc

	for n, c := range d.Configs {
		if !found || n < lowest {
			found, lowest = true, n
			cfg = c
		}
	}
	return &cfg, nil
}

func findInterface(c *gousb.ConfigDesc) (*gousb.InterfaceSetting, error) {
	if len(c.Interfaces) <= 0 {
		return nil, fmt.Errorf("no interface descriptor found")
	}

	infDesc := &(c.Interfaces[0])

	if len(infDesc.AltSettings) <= 0 {
		return nil, fmt.Errorf("no interface alternate setting found")
	}

	return &(infDesc.AltSettings[0]), nil
}

func findEndpoints(s *gousb.InterfaceSetting) (*gousb.EndpointDesc, *gousb.EndpointDesc, error) {
	var inFound, outFound = false, false
	var in, out gousb.EndpointDesc

	for _, desc := range s.Endpoints {
		switch desc.Direction {
		case gousb.EndpointDirectionIn:
			inFound, in = true, desc
		case gousb.EndpointDirectionOut:
			outFound, out = true, desc
		}
	}

	switch {
	case !inFound && !outFound:
		return nil, nil, fmt.Errorf("no endpoint found")
	case !inFound:
		return nil, nil, fmt.Errorf("no IN-endpoint found")
	case !outFound:
		return nil, nil, fmt.Errorf("no OUT-endpoint found")
	default:
		return &in, &out, nil
	}
}

type errors []error

func (es errors) Error() string {
	var ss []string
	for _, e := range es {
		ss = append(ss, e.Error())
	}
	return strings.Join(ss, "\n")
}

type accessoryModeStack struct {
	Context     *gousb.Context
	Device      *gousb.Device
	Config      *gousb.Config
	Interface   *gousb.Interface
	InEndpoint  *gousb.InEndpoint
	OutEndpoint *gousb.OutEndpoint
	ReadStream  *gousb.ReadStream
}

func (s *accessoryModeStack) close() error {
	var e error
	var errs errors

	if s.ReadStream != nil {
		e = s.ReadStream.Close()
		if e != nil {
			errs = append(errs, e)
		}
	}

	if s.Interface != nil {
		s.Interface.Close()
	}

	if s.Config != nil {
		e = s.Config.Close()
		if e != nil {
			errs = append(errs, e)
		}
	}

	if s.Device != nil {
		e = s.Device.Close()
		if e != nil {
			errs = append(errs, e)
		}
	}

	if s.Context != nil {
		e = s.Context.Close()
		if e != nil {
			errs = append(errs, e)
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func openStack(i deviceIdentity) (*accessoryModeStack, error) {
	var err error
	var stack accessoryModeStack
	defer func() {
		if err != nil {
			log.Printf("Cannot open stack: %v, %v", i, err)
			_ = stack.close()
		}
	}()

	stack.Context = gousb.NewContext()

	var devDesc *gousb.DeviceDesc
	ds, err := stack.Context.OpenDevices(func(d *gousb.DeviceDesc) bool {
		if i.match(d) {
			devDesc = d // remember for later inspection
			return true
		}
		return false
	})

	if err != nil {
		for _, d := range ds {
			_ = d.Close()
		}
		return nil, err
	}

	if len(ds) < 1 {
		for _, d := range ds {
			_ = d.Close()
		}
		err = fmt.Errorf("no device found: %v", i)
		return nil, err
	}

	if len(ds) > 1 {
		for _, d := range ds {
			_ = d.Close()
		}
		err = fmt.Errorf("more than one device found: %v", i)
		return nil, err
	}

	stack.Device = ds[0]

	cfgDesc, err := findConfig(devDesc)
	if err != nil {
		return nil, err
	}

	stack.Config, err = stack.Device.Config(cfgDesc.Number)
	if err != nil {
		return nil, err
	}

	infSetting, err := findInterface(cfgDesc)
	if err != nil {
		return nil, err
	}

	stack.Interface, err = stack.Config.Interface(infSetting.Number, infSetting.Alternate)
	if err != nil {
		return nil, err
	}

	epinDesc, epoutDesc, err := findEndpoints(infSetting)
	if err != nil {
		return nil, err
	}

	stack.InEndpoint, err = stack.Interface.InEndpoint(epinDesc.Number)
	if err != nil {
		return nil, err
	}

	stack.OutEndpoint, err = stack.Interface.OutEndpoint(epoutDesc.Number)
	if err != nil {
		return nil, err
	}

	stack.ReadStream, err = stack.InEndpoint.NewStream(epinDesc.MaxPacketSize, 2)
	if err != nil {
		return nil, err
	}

	return &stack, nil
}

func controlRequestIn(d *gousb.Device, request uint8, val, idx uint16, data []byte) int {
	x, err := d.Control(gousb.ControlIn|gousb.ControlVendor, request, val, idx, data)
	if err != nil {
		panic(err)
	}
	return x
}

func controlRequestOut(d *gousb.Device, request uint8, val, idx uint16, data []byte) int {
	x, err := d.Control(gousb.ControlOut|gousb.ControlVendor, request, val, idx, data)
	if err != nil {
		panic(err)
	}
	return x
}

const (
	aoaManufacturer    = "Softcom"
	aoaModel           = "Moonshot"
	aoaDescription     = "4-4-2 Fingerprint Scanner"
	aoaProtocolVersion = "1"
	aoaUri             = "https://softcom.ng"
	aoaSerialNumber    = "0123456789"
)

func switchToAccessoryMode(d *gousb.Device) (err error) {
	defer func() {
		e := recover()
		if e != nil {
			err = e.(error)
		} else {
			err = nil
		}
	}()

	version := controlRequestIn(d, 51, 0, 0, []byte{0x00, 0x00})
	if !id(version).in(1, 2) {
		panic(fmt.Errorf("invalid AOA version number: %v", version))
	}

	controlRequestOut(d, 52, 0, 0, []byte(aoaManufacturer+"\x00"))
	controlRequestOut(d, 52, 0, 1, []byte(aoaModel+"\x00"))
	controlRequestOut(d, 52, 0, 2, []byte(aoaDescription+"\x00"))
	controlRequestOut(d, 52, 0, 3, []byte(aoaProtocolVersion+"\x00"))
	controlRequestOut(d, 52, 0, 4, []byte(aoaUri+"\x00"))
	controlRequestOut(d, 52, 0, 5, []byte(aoaSerialNumber+"\x00"))
	controlRequestOut(d, 53, 0, 0, nil)
	return nil
}

func requestSwitch(i deviceIdentity) error {
	ctx := gousb.NewContext()
	defer func() { _ = ctx.Close() }()

	ds, err := ctx.OpenDevices(i.match)
	for _, d := range ds {
		//noinspection GoDeferInLoop
		defer func() { _ = d.Close() }()
	}
	if err != nil {
		return err
	}

	if len(ds) < 1 {
		return fmt.Errorf("no device found: %v", i)
	}

	if len(ds) > 1 {
		return fmt.Errorf("more than one device found: %v", i)
	}

	return switchToAccessoryMode(ds[0])
}

var currentDeviceMap = make(deviceMap)

func openAccessoryModeStack() *accessoryModeStack {
	for {
		m, identityOfAccessoryMode, identityToSwitch :=
				updateDeviceMap(mapDevices(), currentDeviceMap)
		currentDeviceMap = m

		if !identityOfAccessoryMode.nil() {
			stack, err := openStack(identityOfAccessoryMode)
			if err == nil {
				log.Printf("Accessory mode opened: %v", identityOfAccessoryMode)
				return stack
			}
			log.Printf("Cannot open accessory mode: %v, %v", identityOfAccessoryMode, err)
			currentDeviceMap[identityOfAccessoryMode] = historyOpenFailed
		}

		if !identityToSwitch.nil() {
			log.Printf("Requesting switch: %v", identityToSwitch)
			err := requestSwitch(identityToSwitch)
			if err != nil {
				log.Printf("Cannot switch to accessory mode: %v, %v", identityToSwitch, err)
				currentDeviceMap[identityToSwitch] = historySwitchFailed
			} else {
				log.Printf("Switch to accessory mode requested: %v", identityToSwitch)
				currentDeviceMap[identityToSwitch] = historySwitchRequested

				log.Print("Wait 1 second for it to come on bus again")
				time.Sleep(1 * time.Second)
			}
		} else {
			// Nothing to switch, wait a bit before checking again.
			time.Sleep(2 * time.Second)
		}
	}
}

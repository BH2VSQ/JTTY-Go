//go:build windows

package audio

import (
	"fmt"
	"strings"

	"github.com/degubites/go-wca/pkg/wca"
	"github.com/go-ole/go-ole"
)

type wcaManager struct{}

func NewManager() Manager { return wcaManager{} }

func (wcaManager) enum(flow wca.EDataFlow) ([]Device, error) {
	if err := initializeCOM(); err != nil {
		return nil, err
	}
	defer ole.CoUninitialize()

	var enumerator *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_INPROC_SERVER, wca.IID_IMMDeviceEnumerator, &enumerator); err != nil {
		return nil, err
	}
	defer enumerator.Release()

	var collection *wca.IMMDeviceCollection
	if err := enumerator.EnumAudioEndpoints(uint32(flow), wca.DEVICE_STATE_ACTIVE, &collection); err != nil {
		return nil, err
	}
	defer collection.Release()

	var count uint32
	if err := collection.GetCount(&count); err != nil {
		return nil, err
	}
	devices := make([]Device, 0, count)
	for i := uint32(0); i < count; i++ {
		var dev *wca.IMMDevice
		if err := collection.Item(i, &dev); err != nil {
			continue
		}

		var id string
		if err := dev.GetId(&id); err != nil {
			dev.Release()
			continue
		}

		name := id
		if friendly, err := audioDeviceFriendlyName(dev); err == nil && strings.TrimSpace(friendly) != "" {
			name = friendly
		}
		devices = append(devices, Device{
			ID:          id,
			Name:        name,
			IsInput:     flow == wca.ECapture,
			IsOutput:    flow == wca.ERender,
			SampleRates: []int{44100, 48000, 96000},
		})
		dev.Release()
	}
	return devices, nil
}

func audioDeviceFriendlyName(dev *wca.IMMDevice) (string, error) {
	var store *wca.IPropertyStore
	if err := dev.OpenPropertyStore(0, &store); err != nil {
		return "", err
	}
	defer store.Release()

	var value wca.PROPVARIANT
	if err := store.GetValue(&wca.PKEY_Device_FriendlyName, &value); err != nil {
		return "", err
	}
	defer ole.VariantClear(&value.VARIANT)
	name := strings.TrimSpace(value.String())
	if name == "" {
		return "", fmt.Errorf("friendly name is empty")
	}
	return name, nil
}

func (m wcaManager) Inputs() ([]Device, error)  { return m.enum(wca.ECapture) }
func (m wcaManager) Outputs() ([]Device, error) { return m.enum(wca.ERender) }
func (wcaManager) NewCapture() Source           { return &wasapiCapture{} }
func (wcaManager) NewOutput() Output            { return &wasapiOutput{} }

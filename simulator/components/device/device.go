package device

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/events"
	res "github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/util"

	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/classes"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/models"
)

type Device struct {
	State           int                      `json:"-"`
	Exit            chan struct{}            `json:"-"`
	IntervalChanged chan struct{}            `json:"-"` // Signal to reset ticker when interval changes
	Id              int                      `json:"id"`
	Info            models.InformationDevice `json:"info"`
	Class           classes.Class            `json:"-"`
	Resources       *res.Resources           `json:"-"`
	Mutex           sync.Mutex               `json:"-"`
	EventBroker     *events.EventBroker      `json:"-"`
}

// *******************Intern func*******************/
func (d *Device) Run() {

	defer d.Resources.ExitGroup.Done()

	d.OtaaActivation()

	// Initialize the interval change channel if not already done
	if d.IntervalChanged == nil {
		d.IntervalChanged = make(chan struct{}, 1)
	}

	ticker := time.NewTicker(d.Info.Configuration.SendInterval)
	defer ticker.Stop()

	for {

		select {

		case <-ticker.C:
			break

		case <-d.IntervalChanged:
			// Interval was changed via downlink, reset the ticker
			ticker.Stop()
			ticker = time.NewTicker(d.Info.Configuration.SendInterval)
			slog.Debug("send interval updated", "component", "device", "dev_eui", d.Info.DevEUI, "interval", d.Info.Configuration.SendInterval)
			d.emitEvent(events.EventStatus, map[string]string{"status": fmt.Sprintf("send interval updated to %v", d.Info.Configuration.SendInterval)})
			continue

		case <-d.Exit:
			slog.Debug("device turned off", "component", "device", "dev_eui", d.Info.DevEUI)
			d.emitEvent(events.EventStatus, map[string]string{"status": "turned off"})
			return
		}

		if d.CanExecute() {

			if d.Info.Status.Joined {

				if d.Info.Configuration.SupportedClassC {
					d.SwitchClass(classes.ClassC)
				} else if d.Info.Configuration.SupportedClassB {
					d.SwitchClass(classes.ClassB)
				}

				d.Execute()

			} else {
				d.OtaaActivation()
			}

		}

	}

}

func (d *Device) modeToString() string {

	switch d.Info.Status.Mode {

	case util.Normal:
		return "Normal"

	case util.Retransmission:
		return "Retransmission"

	case util.FPending:
		return "FPending"

	case util.Activation:
		return "Activation"

	default:
		return ""

	}
}

func (d *Device) emitEvent(eventType string, extra map[string]string) {
	if d.EventBroker == nil {
		return
	}
	d.EventBroker.PublishDeviceEvent(d.Info.DevEUI.String(), events.DeviceEvent{
		DevEUI:  d.Info.DevEUI.String(),
		DevName: d.Info.Name,
		Type:    eventType,
		Class:   d.Class.ToString(),
		Extra:   extra,
	})
}

func (d *Device) emitUplinkEvent(fCnt uint32, fPort uint8, dr int, freq uint32, payload string, gwID string) {
	if d.EventBroker == nil {
		return
	}
	d.EventBroker.PublishDeviceEvent(d.Info.DevEUI.String(), events.DeviceEvent{
		DevEUI:    d.Info.DevEUI.String(),
		DevName:   d.Info.Name,
		Type:      events.EventUp,
		FCnt:      &fCnt,
		FPort:     &fPort,
		DR:        &dr,
		Frequency: &freq,
		Payload:   payload,
		Class:     d.Class.ToString(),
		GatewayID: gwID,
	})
}

func (d *Device) emitErrorEvent(err error) {
	if d.EventBroker == nil {
		return
	}
	d.EventBroker.PublishDeviceEvent(d.Info.DevEUI.String(), events.DeviceEvent{
		DevEUI:  d.Info.DevEUI.String(),
		DevName: d.Info.Name,
		Type:    events.EventError,
		Class:   d.Class.ToString(),
		Extra:   map[string]string{"error": err.Error()},
	})
}

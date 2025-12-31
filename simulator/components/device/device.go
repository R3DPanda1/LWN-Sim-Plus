package device

import (
	"fmt"
	"sync"
	"time"

	c "github.com/R3DPanda1/LWN-Sim-Plus/simulator/console"
	res "github.com/R3DPanda1/LWN-Sim-Plus/simulator/resources"

	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/classes"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/models"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/util"
	"github.com/R3DPanda1/LWN-Sim-Plus/socket"
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
	Console         c.Console                `json:"-"`
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
			d.Print(fmt.Sprintf("Send interval updated to %v", d.Info.Configuration.SendInterval), nil, util.PrintBoth)
			continue

		case <-d.Exit:
			d.Print("Turn OFF", nil, util.PrintBoth)
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

func (d *Device) Print(content string, err error, printType int) {

	now := time.Now()
	message := ""
	messageLog := ""
	event := socket.EventDev
	class := d.Class.ToString()
	mode := d.modeToString()

	if err == nil {
		message = fmt.Sprintf("[ %s ] DEV[%s] |%s| {%s}: %s", now.Format(time.Stamp), d.Info.Name, mode, class, content)
		messageLog = fmt.Sprintf("DEV[%s] |%s| {%s}: %s", d.Info.Name, mode, class, content)
	} else {
		message = fmt.Sprintf("[ %s ] DEV[%s] |%s| {%s} [ERROR]: %s", now.Format(time.Stamp), d.Info.Name, mode, class, err)
		messageLog = fmt.Sprintf("DEV[%s] |%s| {%s} [ERROR]: %s", d.Info.Name, mode, class, err)
		event = socket.EventError
	}

	data := socket.ConsoleLog{
		Name: d.Info.Name,
		Msg:  message,
	}

	switch printType {
	case util.PrintBoth:
		d.Console.PrintSocket(event, data)
		d.Console.PrintLog(messageLog)
	case util.PrintOnlySocket:
		d.Console.PrintSocket(event, data)
	case util.PrintOnlyConsole:
		d.Console.PrintLog(messageLog)
	}
}

package main

import (
	"net"

	log "github.com/sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/foogod/go-powerwall"
)

type powerwallCollector struct{
	pw *powerwall.Client
	metrics map[string]*prometheus.Desc
}

func NewPowerwallCollector(client *powerwall.Client) *powerwallCollector {
	c := powerwallCollector{
		pw: client,
		metrics: make(map[string]*prometheus.Desc),
	}
	c.newDesc("info", "Device Information", []string{"version", "git_hash"})
	c.newDesc("uptime_seconds", "Seconds since last reboot", nil)
	c.newDesc("commission_count", "Number of config changes since last reboot", nil)
	c.newDesc("charge_ratio", "Total amount of charge", nil)
	c.newDesc("reserve_ratio", "Amount of charge reserved for backup use", nil)
	c.newDesc("operation_mode", "Operational Mode", []string{"mode"})
	c.newDesc("sitemaster_running", "Is powerwall in running or stopped state?", nil)
	c.newDesc("sitemaster_connected", "Is powerwall connected to Tesla?", nil)
	c.newDesc("power_supply_mode", "Is powerwall in 'power supply' mode?", nil)
	c.newDesc("sitemaster_busy", "Is sitemaster performing some operation which should not be interrupted by stop/reboot?", []string{"reason"})
	c.newDesc("problems_detected_count", "Number of problems currently reported", nil)

	// system status
	c.newDesc("full_pack_joules", "Total capacity of all batteries", nil)
	c.newDesc("remaining_joules", "Remaining charge in all batteries", nil)
	c.newDesc("island_state", "Whether powerwall is running in island mode or connected to grid", []string{"state"})

	// battery info
	c.newDesc("battery_info", "Battery Information", []string{"serial", "partno", "version"})
	c.newDesc("battery_full_pack_joules", "Total battery capacity", []string{"serial"})
	c.newDesc("battery_remaining_joules", "Remaining charge", []string{"serial"})
	c.newDesc("battery_output_volts", "Battery voltage", []string{"serial"})
	c.newDesc("battery_output_amps", "Battery current flow (positive is discharging, negative is charging)", []string{"serial"})
	c.newDesc("battery_output_hz", "Battery output frequency", []string{"serial"})
	c.newDesc("battery_charged_joules_total", "Total amount of energy charged over battery's lifetime", []string{"serial"})
	c.newDesc("battery_discharged_joules_total", "Total amount of energy discharged over battery's lifetime", []string{"serial"})
	c.newDesc("battery_off_grid", "Is battery disconnected from the grid?", []string{"serial"})
	c.newDesc("battery_island_state", "Is battery running in islanded state?", []string{"serial"})
	c.newDesc("battery_wobble_detected", "Is frequency wobble detected?", []string{"serial"})
	c.newDesc("battery_charge_power_clamped", "Has charging power been clamped?", []string{"serial"})
	c.newDesc("battery_backup_ready", "Is battery available for backup use?", []string{"serial"})
	c.newDesc("battery_pinv_state", "Battery power inverter state", []string{"serial", "state"})
	c.newDesc("battery_pinv_grid_state", "Battery power grid state", []string{"serial", "state"})
	c.newDesc("battery_opseq_state", "Battery power grid state", []string{"serial", "state"})

	// aggregates
	c.newDesc("instant_power_watts", "Instant Power (W)", []string{"category"})
	c.newDesc("instant_reactive_power_watts", "Instant Reactive Power (W)", []string{"category"})
	c.newDesc("instant_apparent_power_watts", "Instant Apparent Power (W)", []string{"category"})
	c.newDesc("frequency_hz", "AC Frequency (Hz)", []string{"category"})
	c.newDesc("exported_joules_total", "Energy Exported", []string{"category"}) //TODO: check units
	c.newDesc("imported_joules_total", "Energy Imported", []string{"category"}) //TODO: check units
	c.newDesc("instant_average_volts", "Instant Average Voltage", []string{"category"})
	c.newDesc("instant_average_amps", "Instant Average Current", []string{"category"})
	c.newDesc("instant_total_amps", "Instant Total Current", []string{"category"})

	// meter devices
	c.newDesc("dev_instant_power_watts", "Instant Power (W)", []string{"category", "tyoe", "serial"})
	c.newDesc("dev_instant_reactive_power_watts", "Instant Reactive Power (W)", []string{"category", "tyoe", "serial"})
	c.newDesc("dev_instant_apparent_power_watts", "Instant Apparent Power (W)", []string{"category", "tyoe", "serial"})
	c.newDesc("dev_frequency_hz", "AC Frequency (Hz)", []string{"category", "tyoe", "serial"})
	c.newDesc("dev_exported_joules_total", "Energy Exported", []string{"category", "tyoe", "serial"}) //TODO: check units
	c.newDesc("dev_imported_joules_total", "Energy Imported", []string{"category", "tyoe", "serial"}) //TODO: check units
	c.newDesc("dev_instant_average_volts", "Instant Average Voltage", []string{"category", "tyoe", "serial"})
	c.newDesc("dev_instant_average_amps", "Instant Average Current", []string{"category", "tyoe", "serial"})
	c.newDesc("dev_instant_total_amps", "Instant Total Current", []string{"category", "tyoe", "serial"})

	// network interfaces
	c.newDesc("network_enabled", "Is network interface enabled?", []string{"type", "name"})
	c.newDesc("network_active", "Is network interface active?", []string{"type", "name"})
	c.newDesc("network_primary", "Is this the primary network interface?", []string{"type", "name"})
	c.newDesc("network_state", "Current state and reason for last state change", []string{"type", "name", "state", "reason"})
	c.newDesc("network_signal_strength", "Wireless signal strength", []string{"type", "name"})

	return &c
}

func (c *powerwallCollector) setGaugeBool(ch chan<- prometheus.Metric, name string, value bool, labels ...string) {
	v := float64(0)
	if value {
		v = 1.0
	}
	c.setGauge64(ch, name, v, labels...)
}

func (c *powerwallCollector) setCounter64(ch chan<- prometheus.Metric, name string, value float64, labels ...string) {
	ch <- prometheus.MustNewConstMetric(c.metrics[name], prometheus.CounterValue, value, labels...)
}

func (c *powerwallCollector) setCounter(ch chan<- prometheus.Metric, name string, value float32, labels ...string) {
	c.setCounter64(ch, name, float64(value), labels...)
}

func (c *powerwallCollector) Collect(ch chan<- prometheus.Metric) {
	log.Debug("Collecting metrics...")

	status, err := c.pw.GetStatus()
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Error fetching status info")
		if _, ok := err.(net.Error); ok {
			return
		}
	} else {
		c.setGauge(ch, "info", 1, status.Version, status.GitHash)
		c.setCounter64(ch, "uptime_seconds", status.UpTime.Seconds())
		c.setCounter64(ch, "commission_count", float64(status.CommissionCount))
	}

	soe, err := c.pw.GetSOE()
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Error fetching SOE info")
		if _, ok := err.(net.Error); ok {
			return
		}
	} else {
		c.setGauge(ch, "charge_ratio", soe.Percentage / 100)
	}

	opdata, err := c.pw.GetOperation()
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Error fetching operation info")
		if _, ok := err.(net.Error); ok {
			return
		}
	} else {
		c.setGauge(ch, "operation_mode", 1, opdata.RealMode)
		c.setGauge(ch, "reserve_ratio", opdata.BackupReservePercent / 100)
	}

	sitemaster, err := c.pw.GetSitemaster()
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Error fetching sitemaster info")
		if _, ok := err.(net.Error); ok {
			return
		}
	} else {
		c.setGaugeBool(ch, "sitemaster_running", sitemaster.Running)
		c.setGaugeBool(ch, "sitemaster_connected", sitemaster.ConnectedToTesla)
		c.setGaugeBool(ch, "power_supply_mode", sitemaster.PowerSupplyMode)
		if sitemaster.CanReboot != "Yes" {
			c.setGauge(ch, "sitemaster_busy", 1, sitemaster.CanReboot)
		}
	}

	problems, err := c.pw.GetProblems()
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Error fetching troubleshooting problems info")
		if _, ok := err.(net.Error); ok {
			return
		}
	} else {
		c.setGauge64(ch, "problems_detected_count", float64(len(problems.Problems)))
	}

	sysstatus, err := c.pw.GetSystemStatus()
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Error fetching system_status info")
		if _, ok := err.(net.Error); ok {
			return
		}
	} else {
		c.setGauge(ch, "full_pack_joules", sysstatus.NominalFullPackEnergy * 3600)
		c.setGauge(ch, "remaining_joules", sysstatus.NominalEnergyRemaining * 3600)
		c.setGauge(ch, "island_state", 1, sysstatus.SystemIslandState)

		for _, block := range sysstatus.BatteryBlocks {
			serial := block.PackageSerialNumber
			c.setGauge(ch, "battery_info", 1, serial, block.PackagePartNumber, block.Version)
			c.setGauge(ch, "battery_full_pack_joules", block.NominalFullPackEnergy * 3600, serial)
			c.setGauge(ch, "battery_remaining_joules", block.NominalEnergyRemaining * 3600, serial)
			c.setGauge(ch, "battery_output_volts", block.VOut, serial)
			c.setGauge(ch, "battery_output_amps", block.IOut, serial)
			c.setGauge(ch, "battery_output_hz", block.FOut, serial)
			c.setCounter64(ch, "battery_charged_joules_total", float64(block.EnergyCharged) * 3600, serial)
			c.setCounter64(ch, "battery_discharged_joules_total", float64(block.EnergyDischarged) * 3600, serial)
			c.setGaugeBool(ch, "battery_off_grid", block.OffGrid, serial)
			c.setGaugeBool(ch, "battery_island_state", block.VfMode, serial)
			c.setGaugeBool(ch, "battery_wobble_detected", block.WobbleDetected, serial)
			c.setGaugeBool(ch, "battery_charge_power_clamped", block.ChargePowerClamped, serial)
			c.setGaugeBool(ch, "battery_backup_ready", block.BackupReady, serial)
			c.setGauge(ch, "battery_pinv_state", 1, serial, block.PinvState)
			c.setGauge(ch, "battery_pinv_grid_state", 1, serial, block.PinvGridState)
			c.setGauge(ch, "battery_opseq_state", 1, serial, block.OpSeqState)
		}
	}

	aggs, err := c.pw.GetMetersAggregates()
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Error fetching meter aggregates info")
		if _, ok := err.(net.Error); ok {
			return
		}
	} else {
		for cat, data := range *aggs {
			c.setGauge(ch, "instant_power_watts", data.InstantPower, cat)
			c.setGauge(ch, "instant_reactive_power_watts", data.InstantReactivePower, cat)
			c.setGauge(ch, "instant_apparent_power_watts", data.InstantApparentPower, cat)
			if data.Frequency != 0 {
				c.setGauge(ch, "frequency_hz", data.Frequency, cat)
			}
			c.setCounter64(ch, "exported_joules_total", float64(data.EnergyExported) * 3600, cat)
			c.setCounter64(ch, "imported_joules_total", float64(data.EnergyImported) * 3600, cat)
			c.setGauge(ch, "instant_average_volts", data.InstantAverageVoltage, cat)
			c.setGauge(ch, "instant_average_amps", data.InstantAverageCurrent, cat)
			c.setGauge(ch, "instant_total_amps", data.InstantTotalCurrent, cat)

			devs, err := c.pw.GetMeters(cat)
			if err != nil {
				log.WithFields(log.Fields{"cat": cat, "err": err}).Error("Error fetching detailed meter info")
			} else {
				for _, dev := range *devs {
					devtype := dev.Type
					serial := dev.Connection.DeviceSerial
					data := dev.CachedReadings
					c.setGauge(ch, "dev_instant_power_watts", data.InstantPower, cat, devtype, serial)
					c.setGauge(ch, "dev_instant_reactive_power_watts", data.InstantReactivePower, cat, devtype, serial)
					c.setGauge(ch, "dev_instant_apparent_power_watts", data.InstantApparentPower, cat, devtype, serial)
					if data.Frequency != 0 {
						c.setGauge(ch, "dev_frequency_hz", data.Frequency, cat, devtype, serial)
					}
					c.setCounter64(ch, "dev_exported_joules_total", float64(data.EnergyExported) * 3600, cat, devtype, serial)
					c.setCounter64(ch, "dev_imported_joules_total", float64(data.EnergyImported) * 3600, cat, devtype, serial)
					c.setGauge(ch, "dev_instant_average_volts", data.InstantAverageVoltage, cat, devtype, serial)
					c.setGauge(ch, "dev_instant_average_amps", data.InstantAverageCurrent, cat, devtype, serial)
					c.setGauge(ch, "dev_instant_total_amps", data.InstantTotalCurrent, cat, devtype, serial)
				}
			}
		}
	}

	nets, err := c.pw.GetNetworks()
	if err != nil {
		log.WithFields(log.Fields{"err": err}).Error("Error fetching networks info")
		if _, ok := err.(net.Error); ok {
			return
		}
	} else {
		for _, net := range *nets {
			name := net.NetworkName
			nettype := net.Interface
			c.setGaugeBool(ch, "network_enabled", net.Enabled, nettype, name)
			c.setGaugeBool(ch, "network_active", net.Active, nettype, name)
			c.setGaugeBool(ch, "network_primary", net.Primary, nettype, name)
			iface := net.IfaceNetworkInfo
			if iface.NetworkName != "" {
				c.setGauge(ch, "network_state", 1, nettype, name, iface.State, iface.StateReason)
				if iface.SignalStrength != 0 {
					c.setGauge64(ch, "network_signal_strength", float64(iface.SignalStrength), nettype, name)
				}
			}
		}
	}
}

func (c *powerwallCollector) newDesc(name string, desc string, labels []string) {
	c.metrics[name] = prometheus.NewDesc(exporterName + "_" + name, desc, labels, nil)
}

func (c *powerwallCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range c.metrics {
		ch <- desc
	}
}

func (c *powerwallCollector) setGauge64(ch chan<- prometheus.Metric, name string, value float64, labels ...string) {
	ch <- prometheus.MustNewConstMetric(c.metrics[name], prometheus.GaugeValue, value, labels...)
}

func (c *powerwallCollector) setGauge(ch chan<- prometheus.Metric, name string, value float32, labels ...string) {
	c.setGauge64(ch, name, float64(value), labels...)
}

// grid faults

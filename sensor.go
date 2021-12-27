package main

import (
	"flag"
	"fmt"
	"github.com/golang/glog"
	"os"
	"snowsensor/conf"
	"snowsensor/protocol"
)

func main() {
	confFile := flag.String("config", "sensor.conf", "configuration file")
	cmd := flag.String("command", "measure", "measure, laseron, laseroff")
	help := flag.Bool("help", false, "show usage")
	flag.Parse()

	if *help {
		flag.Usage()
		return
	}

	cfg, isDefault := conf.GetConfig(*confFile)
	if isDefault {
		cfg.Store()
	}

	switch *cmd {
	case "info":
		cmdInfo(cfg)
	case "laseron":
		cmdLaser(true, cfg)
	case "laseroff":
		cmdLaser(false, cfg)
	case "measure":
		fallthrough
	default:
		cmdMeasure(cfg)
	}
}

func cmdInfo(cfg conf.Config) {
	pr := mustGetProto(cfg)
	defer pr.Close()

	info, err := pr.GetInfo()
	if err != nil {
		glog.Exit(err)
	}

	fmt.Printf("Name:   %v\n", info.SensorName)
	fmt.Printf("Type:   %v\n", info.SensorType)
	fmt.Printf("Group:  %v\n", info.SensorGroup)
	fmt.Printf("Serial: %v\n", info.SerialNumber)
	fmt.Printf("Firmware v%v.%v.%v; week %v; year %v\n",
		info.FirmwareMajor, info.FirmwareMinor, info.FirmwareRevision,
		info.FirmwareCalendarWeek, info.FirmwareYear)
}

func cmdLaser(on bool, cfg conf.Config) {
	pr := mustGetProto(cfg)
	defer pr.Close()

	err := pr.SetLaser(on)
	if err != nil {
		glog.Exitf(err.Error())
	}

	lstr := "off"
	if on {
		lstr = "on"
	}
	fmt.Printf("laser has been switched %v\n", lstr)
}

func cmdMeasure(cfg conf.Config) {
	pr := mustGetProto(cfg)
	defer pr.Close()

	height, when, err := pr.GetMeasurement()
	if err != nil {
		glog.Exitf(err.Error())
	}

	fmt.Printf("timestamp=\"%v\" value=\"%.1f\"\n", when, height)
}

func mustGetProto(cfg conf.Config) *protocol.Proto {
	pr := protocol.InitProto(protocol.P_WENGLOR, cfg)
	if pr == nil {
		os.Exit(1)
	}
	return pr
}

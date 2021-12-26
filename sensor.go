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

func cmdLaser(on bool, cfg conf.Config) {
	pr := protocol.InitProto(protocol.P_WENGLOR, cfg)
	if pr == nil {
		os.Exit(1)
	}
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
	pr := protocol.InitProto(protocol.P_WENGLOR, cfg)
	if pr == nil {
		os.Exit(1)
	}
	defer pr.Close()

	height, when, err := pr.GetMeasurement()
	if err != nil {
		glog.Exitf(err.Error())
	}

	fmt.Printf("timestamp=\"%v\" value=\"%.1f\"\n", when, height)
}

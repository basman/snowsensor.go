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

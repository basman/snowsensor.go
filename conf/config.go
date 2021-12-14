package conf

import (
	"bufio"
	"fmt"
	"github.com/golang/glog"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const CONFIG_FILE = "sensor.conf"

type Config struct {
	Retry     int32
	Zeroline  float32
	Scale     float32
	Offset    float32
	Warmup_ms int32
	Host      string // was: char[128]
	Port      string // was: char[6]
}

var defaultConf = Config{
	Retry:     4,
	Zeroline:  2200,
	Scale:     0.1,
	Offset:    0,
	Warmup_ms: 1000,
	Host:      "192.168.0.44",
	Port:      "10001",
}

var cfg *Config

func GetConfig(filename string) (Config, bool) {
	isDefault := false
	if cfg == nil {
		cfg, isDefault = load(filename)
	}

	return *cfg, isDefault
}

func (c *Config)Store() {
	f, err := os.OpenFile(CONFIG_FILE, os.O_CREATE | os.O_TRUNC | os.O_WRONLY, 0644)
	if err != nil {
		glog.Warningf("failed to store configuration: %v", err)
		return
	}

	w := bufio.NewWriter(f)

	o := func(name string, val interface{}) {
		_, err := fmt.Fprintf(w, "%v=%v\n", name, val)
		if err != nil {
			glog.Errorf("failed to write config setting '%v'='%v'. error: %v\n", name, val, err)
		}
	}

	o("retry", c.Retry)
	o("offset", c.Offset)
	o("scale", c.Scale)
	o("zeroline", c.Zeroline)
	o("host", c.Host)
	o("port", c.Port)
	o("warmup", c.Warmup_ms)

	w.Flush()
	f.Close()
}

func load(filename string) (*Config, bool) {
	f, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			glog.Warningf("config file %v not found. using default configuration.", filename)
			return &defaultConf, true
		}
		glog.Exitf("failed to open existing config file %v: %v", filename, err)
	}
	defer f.Close()

	var conf Config

	commentOrEmptyLineRe := regexp.MustCompile(`^\s*(#.*)?$`)
	nameValueRe := regexp.MustCompile(`^\s*(\S+)\s*=\s*(.*?)\s*$`)

	r := bufio.NewScanner(f)
	for r.Scan() {
		l := r.Text()

		if commentOrEmptyLineRe.MatchString(l) {
			continue
		}

		groups := nameValueRe.FindStringSubmatch(l)
		if groups != nil && len(groups) == 3 {
			name, value := groups[1], groups[2]
			if err := conf.SetValue(name, value); err != nil {
				glog.Errorf("invalid configuration entry '%v': ", err)
				os.Exit(1)
			}
		}
	}

	return &conf, false
}

func (conf *Config) SetValue(name, value string) error {
	var err error

	switch strings.ToLower(name) {
	case "scale":
		conf.Scale, err = parseFloat32(value)
	case "offset":
		conf.Offset, err = parseFloat32(value)
	case "zeroline":
		conf.Zeroline, err = parseFloat32(value)
	case "retry":
		conf.Retry, err = parseInt32(value)
	case "warmup":
		conf.Warmup_ms, err = parseInt32(value)
	case "host":
		conf.Host = value
	case "port":
		conf.Port = value
	case "logfile":
		fallthrough
	case "loglevel":
		glog.Warningf("Warning: ignoring obsolete config setting '%v'", name)
	default:
		glog.Exitf("unknown config setting '%v'", name)
	}

	if glog.V(2) {
		glog.Infof("config: %v=%v\n", name, value)
	}
	return err
}

func parseFloat32(value string) (float32, error) {
	v, err := strconv.ParseFloat(value, 32)
	return float32(v), err
}

func parseInt32(value string) (int32, error) {
	v, err := strconv.ParseInt(value, 10, 32)
	return int32(v), err
}

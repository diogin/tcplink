package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

type rule struct {
	Name   string `json:"name"`
	Mode   string `json:"mode"`
	Secret string `json:"secret"`
	Listen string `json:"listen"`
	Target string `json:"target"`
}

func main() {
	var file string
	if len(os.Args) > 1 {
		file = os.Args[1]
	} else {
		exe, err := os.Executable()
		must(err)
		file = path.Dir(strings.Replace(exe, "\\", "/", -1)) + "/config.json"
	}
	conf, err := ioutil.ReadFile(file)
	must(err)
	var rules []rule
	must(json.Unmarshal(conf, &rules))
	for _, link := range rules {
		switch link.Mode {
		case "relay":
			go serveRelay(link.Listen, link.Target)
		case "inner":
			go serveInner(link.Listen, link.Target, link.Secret)
		case "outer":
			go serveOuter(link.Listen, link.Target, link.Secret)
		case "finder":
			go serveFinder(link.Secret, link.Listen, link.Target)
		case "mapper":
			go serveMapper(link.Secret, link.Listen, link.Target)
		case "broker":
			go serveBroker(link.Secret, link.Listen, link.Target)
		case "router":
			go serveRouter(link.Secret, link.Listen, link.Target)
		case "http":
			go serveHttp(link.Listen)
		case "sock":
			go serveSock(link.Listen)
		case "https":
			go serveHttps(link.Listen, link.Target, link.Secret)
		case "socks":
			go serveSocks(link.Listen, link.Target, link.Secret)
		case "agent":
			go serveAgent(link.Listen, link.Secret)
		default:
			fmt.Println("bad mode")
			return
		}
	}
	select {}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

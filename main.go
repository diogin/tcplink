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
	Name string            `json:"name"`
	Mode string            `json:"mode"`
	Args map[string]string `json:"args"`
}

var rulers = map[string]func(args map[string]string){
	"relay":  serveRelay,
	"inner":  serveInner,
	"outer":  serveOuter,
	"finder": serveFinder,
	"mapper": serveMapper,
	"broker": serveBroker,
	"router": serveRouter,
	"http":   serveHttp,
	"sock":   serveSock,
	"https":  serveHttps,
	"socks":  serveSocks,
	"agent":  serveAgent,
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
	for _, rule := range rules {
		if ruler, ok := rulers[rule.Mode]; ok {
			go ruler(rule.Args)
		} else {
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

package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
)

type Properties struct {
	WebIP    string `json:"web-ip"`
	WebPort  int    `json:"web-port"`
	Selected string `json:"selected"`
}

var Config = &Properties{
	WebIP:   "localhost",
	WebPort: 8080,
}

func main() {
	// load properties
	data, err := os.ReadFile("properties.json")
	if err != nil {
		data, err = json.MarshalIndent(Config, "", "    ")

		if err != nil {
			println("marshalling json:" + err.Error())
			return
		}

		os.WriteFile("properties.json", data, 0777)
	}

	err = json.Unmarshal(data, Config)
	if err != nil {
		println("parsing json:" + err.Error())
		return
	}

	err = Config.save()
	if err != nil {
		println("saving config:" + err.Error())
		return
	}

	// load panels
	if err != nil {
		println("loading nodes: " + err.Error())
		return
	}

	// register web api and webserver
	http.Handle("/", http.FileServer(http.Dir("web")))

	go http.ListenAndServe(Config.WebIP+":"+strconv.Itoa(Config.WebPort), nil)

	// listen for input to exit/manage matrix
	println("Launched Matrix32 Server started. Press ENTER to exit.")
	scanner := bufio.NewScanner(bufio.NewReader(os.Stdin))
	for scanner.Scan() {
		return
	}
}

func (p *Properties) save() error {
	// save properties
	json.MarshalIndent(Config, "", "    ")

	data, err := json.MarshalIndent(Config, "", "    ")

	if err != nil {
		return err
	}

	if err := os.WriteFile("properties.json", data, 0777); err != nil {
		return err
	}

	return nil
}

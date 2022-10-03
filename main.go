package main

import "net/http"

type properties struct {
	Addr string `json:"addr"`
}

var config = &properties{
	Addr: "10.0.0.211",
}

func main() {
	http.Handle("/", http.FileServer(http.Dir("web")))

	handlerFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		newAddr := r.URL.Query().Get("addr")

		if newAddr != "" {

		}
	})
	http.Handle("/api/set-board", handlerFunc)
}

func (p *properties) save() {

}

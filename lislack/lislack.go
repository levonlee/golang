package main

import (
	"fmt"
	"github.com/levonlee/golang/ligithub"
	"github.com/levonlee/golang/lislackapi"
	"net/http"
)

func main() {
	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello html")
	})

	ligithub.GithubRoutes()
	lislackapi.SlackRoutes()
	fmt.Println(http.ListenAndServe(":49162", nil))
}

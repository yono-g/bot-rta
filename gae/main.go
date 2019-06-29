package main

import (
	"net/http"

	"github.com/yono-g/bot-rta/app/tasks"
	"google.golang.org/appengine"
)

func main() {
	http.HandleFunc("/tasks/main", tasks.MainTask)

	appengine.Main()
}

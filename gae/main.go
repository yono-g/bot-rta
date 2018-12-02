package main

import (
	"net/http"

	"yono.test/bot-rta/app/tasks"
)

func init() {
	http.HandleFunc("/tasks/main", tasks.MainTask)
}

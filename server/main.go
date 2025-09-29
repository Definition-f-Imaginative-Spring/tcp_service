package main

import (
	"github.com/Definition-f-Imaginative-Spring/tcp_service/server/connection"
)

func main() {
	manage := connection.NewConnectionManager()
	manage.Listen()

}

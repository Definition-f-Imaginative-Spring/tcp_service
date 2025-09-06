package main

import (
	"Chatplus/server/connection"
)

func main() {
	manage := connection.NewConnectionManager()
	manage.Listen()

}

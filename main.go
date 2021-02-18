package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
		return
	}

	mb, err := NewMeetingBot(os.Getenv("server"), os.Getenv("wsserver"), os.Getenv("token"))
	if err != nil {
		fmt.Printf("Error: %+v \n", err)
		return
	}

	for {
		fmt.Println("Connecting...")
		err = mb.Listen()
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		time.Sleep(5 * time.Second)
	}

}

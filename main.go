package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/chofnar/mygarage-offers/execute"
)

const waitTimeMinutes = 2

func main() {
	persist := execute.Execute(nil)
	if persist != nil {
		for {
			fmt.Println("Executed. Waiting for " + strconv.Itoa(waitTimeMinutes) + " minutes")
			time.Sleep(time.Minute * waitTimeMinutes)
			persist = execute.Execute(persist)
		}
	}
}

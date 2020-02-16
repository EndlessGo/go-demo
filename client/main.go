package main

import (
	"fmt"
	"strings"
)

func main()  {
	who := "Hello World!"
	who = strings.Join([]string{who}," ")
	fmt.Println(who)
	fmt.Println("WonderChaos", who)
}
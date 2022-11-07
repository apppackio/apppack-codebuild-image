package build

import "fmt"

const marker = "#*#*#*#*#*# apppack-%s-%s #*#*#*#*#*#\n"

func PrintStartMarker(phase string) {
	fmt.Printf(marker, phase, "start")
}

func PrintEndMarker(phase string) {
	fmt.Printf(marker, phase, "end")
}

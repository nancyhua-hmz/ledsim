package ledsim

import (
	_ "embed"
	"fmt"
)

func LoadLEDs(sys *System) {
	// var (
	// 	scale  = 0.0005 * 2.056422
	// 	origin = [...]float64{
	// 		(1 / (0.0005 * 2.056422)) * -18.04,
	// 		(1 / (0.0005 * 2.056422)) * 9.58,
	// 		(1 / (0.0005 * 2.056422)) * 1,
	// 	}
	// )

	g := newGraph()
	g.populateGraph(sys)
	sys.Normalize()

	for _, led := range sys.LEDs {
		led.Neighbours = g.edges[led]
	}

	// destroy graph
	for k := range g.edges {
		delete(g.edges, k)
	}

	// hint at garbage collector
	g.vertices = nil
	g = nil

	fmt.Println("loaded", len(sys.LEDs), "leds")
}

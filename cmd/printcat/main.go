package main

import "github.com/printcat/printcat/internal/ui/fyneapp"

func main() {
	application := fyneapp.New()
	window := fyneapp.NewWindow(application)
	window.ShowAndRun()
}

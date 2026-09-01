package main

import "github.com/timboli111/PrintCat/internal/ui/fyneapp"

func main() {
	application := fyneapp.New()
	window := fyneapp.NewWindow(application)
	window.ShowAndRun()
}

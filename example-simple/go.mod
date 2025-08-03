module github.com/blackorder/reloader/example-simple

go 1.24.5

require github.com/blackorder/reloader v0.0.0

replace github.com/blackorder/reloader => ../

require (
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
)

replace github.com/blackorder/reloader => ../

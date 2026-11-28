module zoomriya.com/os/box-desktop

go 1.24

require zoomriya.com/os/localhttp v0.0.0

// The box's own shared code (zoomriya-os/lib), never fetched.
replace zoomriya.com/os/localhttp => ../../../lib/localhttp

package main

import (
	"state-machine-engine/cmd"
	"state-machine-engine/internal/bootstrap"
)

func main() {
	rt := bootstrap.NewDefaultRuntime()
	cmd.Execute(rt)
}

package main

import (
	"emt"
	"emt/example/core"
	"emt/log"
	"emt/registry"
	rg "emt/registry/redis"
	"time"
)

const (
	registryAddr string = "172.28.201.237:6379"
	registryPswd string = "123456"
)

func main() {
	log.Init("emt")
	startServer()
	time.Sleep(time.Second)
}

// go startClient().
// core.TestCliWriteMsg().
// }

func startServer() {
	app := emt.NewApp()

	reg := rg.NewRegistry(
		registry.OptionWithAddr(registryAddr),
		registry.OptionWithPassword(registryPswd))

	if err := app.AddServer(core.DefaultSrv); err != nil {
		panic(err)
	}

	if err := app.AddRPCServer(core.DefaultRPCSrv); err != nil {
		panic(err)
	}

	app.AddRegistry(reg)

	if err := app.Run(); err != nil {
		log.Error(err.Error())
	}
}

/*
func startClient() {
	app := emt.NewApp()

	reg := rg.NewRegistry(
		registry.OptionWithAddr(registryAddr),
		registry.OptionWithPassword(registryPswd))

	app.AddRegistry(reg)

	if err := app.AddClient(core.DefaultCli); err != nil {
		panic(err)
	}

	if err := app.AddRPCClient(core.DefaultRPCCli); err != nil {
		panic(err)
	}

	if err := app.Run(); err != nil {
		log.Error(err.Error())
	}
}
*/

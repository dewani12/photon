package main

import (
	"github.com/dewani12/photon/internal/gateway"
)

func main(){
	cfg,err := gateway.DefaultConfig()
	if err != nil {
		panic(err)
	}
	llmg:= gateway.New(cfg)

	llmg.Start()
}
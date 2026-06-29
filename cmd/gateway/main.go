package main

import (
	"github.com/dewani12/photon/internal/gateway"
)

func main(){
	cfg:=gateway.DefaultConfig()
	llmg:=gateway.New(cfg)

	llmg.Start()
}
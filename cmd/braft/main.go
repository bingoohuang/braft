package main

import (
	"fmt"
	"log"
	"time"

	"github.com/bingoohuang/braft"
	"github.com/bingoohuang/braft/util"
	"github.com/bingoohuang/gg/pkg/flagparse"
	"github.com/bingoohuang/gg/pkg/v"
	"github.com/bingoohuang/golog"
)

type Arg struct {
	Version bool `flag:",v"`
	Init    bool
}

// Usage is optional for customized show.
func (a Arg) Usage() string {
	return fmt.Sprintf(`
Usage of goup:
  -v    bool   show version
  -init bool   create init ctl shell script`)
}

// VersionInfo is optional for customized version.
func (a Arg) VersionInfo() string { return v.Version() }

func main() {
	c := &Arg{}
	flagparse.Parse(c)
	if sleep, _ := time.ParseDuration(util.Env("SLEEP")); sleep > 0 {
		time.Sleep(sleep)
	}

	golog.Setup()

	node, err := braft.NewNode()
	if err != nil {
		log.Fatalf("failed to new node, error: %v", err)
	}
	if err := node.Start(); err != nil {
		log.Fatalf("failed to start node, error: %v", err)
	}

	node.RunHTTP()
}

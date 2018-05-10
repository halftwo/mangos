package main

import (
	"fmt"

	"mangos/xic"
)

func run(engine xic.Engine, args []string) error {
	prx, err := engine.StringToProxy("Demo@tcp++5555")
	if err != nil {
		return err
	}

	quest := xic.NewArguments()
	quest.Set("hello", 1.25)
	quest.Set("world", "All men are created equal")

	var answer xic.Arguments
	answer = xic.NewArguments()
	err = prx.Invoke("echo", quest, &answer)
	fmt.Println(err, answer)

	answer = xic.NewArguments()
	err = prx.Invoke("time", quest, &answer)
	fmt.Println(err, answer)
	return nil
}

func main() {
	xic.Start(run)
}


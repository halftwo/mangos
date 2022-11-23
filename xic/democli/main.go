package main

import (
	"fmt"

	"halftwo/mangos/xic"
)

func run(engine xic.Engine, args []string) error {
	secretBox, err := xic.NewSecretBox("@++=hello:world")
	if err != nil {
		return err
	}
	engine.SetSecretBox(secretBox)

	prx, err := engine.StringToProxy("Demo@tcp++5555")
	if err != nil {
		return err
	}

	quest := xic.NewArguments()
	quest.Set("hello", 1.25)
	quest.Set("world", "All men are created equal")

	answer := xic.NewArguments()
	err = prx.Invoke("echo", quest, &answer)
	fmt.Println(err, answer)

//	quest = xic.NewArguments()
//	answer = xic.NewArguments()
//	err = prx.InvokeAsync("time", quest, &answer)
//	fmt.Println(err, answer)

	var res xic.Result
	for i := 0; i < 10000; i++ {
		quest = xic.NewArguments()
		answer = xic.NewArguments()
		res = prx.InvokeAsync("time", quest, &answer)
		if i % 2000 == 0 {
			res.Wait()
		}
	}
	res.Wait()
	fmt.Println(err, res.Out())

	engine.Shutdown()
	return nil
}

func main() {
	xic.Start(run)
}


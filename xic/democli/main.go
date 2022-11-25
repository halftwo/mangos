package main

import (
	"fmt"
	"strconv"

	"halftwo/mangos/xic"
)

func run(engine xic.Engine, args []string) error {
	secretBox, err := xic.NewSecretBox("@++=hello:world")
	if err != nil {
		return err
	}
	engine.SetSecretBox(secretBox)

	netloc := "Demo@++5555"
	num := 10000
	if len(args) > 1 {
		netloc = fmt.Sprintf("Demo@+%s+5555", args[1])
		if len(args) > 2 {
			num, err = strconv.Atoi(args[2])
		}
	}

	prx, err := engine.StringToProxy(netloc)
	if err != nil {
		return err
	}

	quest := xic.NewArguments()
	quest.Set("hello", 1.25)
	quest.Set("world", "All men are created equal")

	answer := xic.NewArguments()
	err = prx.Invoke("echo", quest, answer)
	fmt.Println(err, answer)

	type TimeAnswer struct {
		Con string `vbs:"con"`
		Strftime map[string]string `vbs:"strftime"`
		Time int `vbs:"time"`
	}
	var tan TimeAnswer
	err = prx.Invoke("time", nil, &tan)
	fmt.Println(err, tan)

	var res xic.Result
	for i := 0; i < num; i++ {
		var answer TimeAnswer
		res = prx.InvokeAsync("time", nil, &answer)
		if i % 500 == 0 {
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


package main

import (
	"fmt"
	"strconv"
	"time"

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

	type EchoAnswer struct {
		A float32 `vbs:"参数1"`
		B string `vbs:"参数2"`
	}
	quest := xic.NewArguments()
	quest.Set("参数1", 1.25)
	quest.Set("参数2", "All men are created equal")

	var echoans EchoAnswer
	err = prx.Invoke("echo", quest, &echoans)
	fmt.Println(err, echoans)

	type TimeAnswer struct {
		Con string `vbs:"con"`
		Strftime map[string]string `vbs:"strftime"`
		Time int `vbs:"time"`
	}
	answer := xic.NewArguments()
	err = prx.Invoke("time", nil, answer)
	fmt.Println(err, answer)

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
	start := time.Now()
	xic.Start(run)
	fmt.Println("Time elapsed:", float32(time.Now().Sub(start))/float32(time.Second))
}


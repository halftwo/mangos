package main

import (
	"fmt"
	"time"
	"mangos/xic"
)

type demoServant struct {
	xic.DefaultServant
	adapter xic.Adapter
}

func newServant(adapter xic.Adapter) *demoServant {
	setting := adapter.Engine().Setting()
	name := setting.Get("demo.name")

	srv := &demoServant{adapter:adapter}
	if name != "" {
		adapter.AddServant(name, srv)
	}
	return srv
}

func (srv *demoServant) Xic_echo(cur xic.Current, in xic.Arguments, out *xic.Arguments) error {
	out.CopyFrom(in)
	return nil
}

type timeInArgs struct {
	Time int64 `vbs:"time,omitempty"`
}

type times struct {
	Utc string `vbs:"utc"`
	Local string `vbs:"local"`
}

type timeOutArgs struct {
	Con string `vbs:"con"`
	Time int64 `vbs:"time"`
	Strftime times `vbs:"strftime"`
}

func (srv *demoServant) Xic_time(cur xic.Current, in timeInArgs, out *timeOutArgs) error {
	var t time.Time
	if in.Time == 0 {
		t = time.Now()
	} else {
		t = time.Unix(in.Time, 0)
	}
	const format = "2006-01-02T03:04:05-07:00 Mon"
	out.Con = cur.Con().String()
	out.Time = t.Unix()
	out.Strftime.Utc = t.UTC().Format(format)
	out.Strftime.Local = t.Format(format)
	return nil
}


func run(engine xic.Engine, args []string) error {
	adapter, err := engine.CreateAdapter("")
	if err != nil {
		return err
	}

	servant := newServant(adapter)
	_, err = adapter.AddServant("Demo", servant)
	if err != nil {
		fmt.Println("ERR", err)
	}
	adapter.Activate()
	engine.WaitForShutdown()
	return nil
}

func main() {
	xic.Start(run)
}


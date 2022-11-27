package main

import (
	"time"
	"halftwo/mangos/xic"
	"halftwo/mangos/dlog"
)

type _DemoServant struct {
	xic.DefaultServant
	adapter xic.Adapter
}

func newServant(adapter xic.Adapter) *_DemoServant {
	setting := adapter.Engine().Setting()
	name := setting.Get("demo.name")

	srv := &_DemoServant{adapter:adapter}
	if name != "" {
		adapter.AddServant(name, srv)
	}
	return srv
}

func (srv *_DemoServant) Xic_echo(cur xic.Current, in xic.Arguments, out *xic.Arguments) error {
	*out = in
//	out.CopyFrom(in)
	return nil
}

type _TimeInArgs struct {
	Time int64 `vbs:"time,omitempty"`
}

type _Times struct {
	Ctime string `vbs:"ctime"`
	Local string `vbs:"local"`
}

type _TimeOutArgs struct {
	Con string `vbs:"con"`
	Time int64 `vbs:"time"`
	Strftime _Times `vbs:"strftime"`
}

func (srv *_DemoServant) Xic_time(cur xic.Current, in _TimeInArgs, out *_TimeOutArgs) error {
	var t time.Time
	if in.Time == 0 {
		t = time.Now()
	} else {
		t = time.Unix(in.Time, 0)
	}
	out.Con = cur.Con().String()
	out.Time = t.Unix()
	out.Strftime.Ctime = t.Format(time.ANSIC)
	out.Strftime.Local = dlog.TimeString(t)
	return nil
}


func run(engine xic.Engine, args []string) error {
	adapter, err := engine.CreateAdapter("")
	if err != nil {
		return err
	}

	servant := newServant(adapter)
	adapter.MustAddServant("Demo", servant)
	adapter.Activate()
	return nil
}

func main() {
	xic.Start(run)
}


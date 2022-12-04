package xic

import (
	"os"
	"fmt"
	"strings"
	"time"
	"os/signal"
	"sync/atomic"
)

func parseArgs() (file string, cfs map[string]string, args []string) {
	args = append(args, os.Args[0])
	i := 1
	for ; i < len(os.Args); i++ {
		s := os.Args[i]

		if s == "--" {
			break
		}
		if (s == "-?" || s == "--help") {
			usage()
			os.Exit(1)
		}

		if strings.HasPrefix(s, "--") {
			if strings.HasPrefix(s, "--xic.conf=") {
				file = s[11:]
				continue
			}
			dot := strings.IndexByte(s[2:], '.')
			if dot > 0 {
				dot += 2
				eq := strings.IndexByte(s[dot+1:], '=')
				if eq > 0 {
					eq += dot + 1
					key := s[:eq]
					value := s[eq+1:]
					cfs[key] = value
					continue
				}
			}
		}

		args = append(args, s)
	}

	args = append(args, os.Args[i:]...)
	return
}

func usage() {
	fmt.Fprintf(os.Stderr, "\nUsage: %s --xic.conf=<config_file> [--AAA.BBB=ZZZ]\n\n",
		os.Args[0])
	os.Exit(1)
}

var started atomic.Int32
func start_setting_signal(entree EntreeFunction, setting Setting, sigFun SignalHandler) (Engine, error) {
	if !started.CompareAndSwap(0, 1) {
		panic("function start_with_setting() can only be called once")
	}

	start_time := time.Now()
	if setting == nil {
		setting = NewSetting()
	}

	configFile, cfs, args := parseArgs()
	if configFile != "" {
		err := setting.LoadFile(configFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, "ERROR:", err)
			return nil, err
		}
	}
	for k, v := range cfs {
		setting.Set(k, v)
	}

	engine := newEngineSetting(setting)
	if sigFun == nil {
		sigFun = engine.sig_handler_routine
	}
	go sigFun(engine.sigChan)

	signal.Notify(engine.sigChan, os.Interrupt)
	install_additional_signals(engine)

	if entree == nil {
		return engine, nil
	}

	err := entree(engine, args)
	if err != nil {
		engine.Shutdown()
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		if configFile == "" && time.Now().Sub(start_time) < time.Second {
			usage()
		}
		engine.WaitForShutdown()
		engine = nil
	}

	return engine, err
}


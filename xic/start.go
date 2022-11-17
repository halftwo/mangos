package xic

import (
	"os"
	"fmt"
	"strings"
	"os/signal"
)

func usage() {
	fmt.Fprintf(os.Stderr, "\nUsage: %s --xic.conf=<config_file> [--AAA.BBB=ZZZ]\n\n",
		os.Args[0])
	os.Exit(1)
}

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

func StartSetting(run EntreeFunction, setting Setting) error {
	if setting == nil {
		setting = NewSetting()
	}

	configFile, cfs, args := parseArgs()
	if configFile != "" {
		err := setting.LoadFile(configFile)
		if err != nil {
			usage()
		}
	}
	for k, v := range cfs {
		setting.Set(k, v)
	}

	// TODO
	engine := newEngineSetting(setting)
	signal.Notify(engine.shutdownChan, os.Interrupt)
	install_additional_signals(engine)
	err := run(engine, args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		usage()
	}
	return err
}

func Start(run EntreeFunction) error {
	return StartSetting(run, nil)
}


package profile

import (
	"emt/log"
	"net/http"

	// pprof.
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"go.uber.org/zap"
)

var (
	DefaultCPUf string
	DefaultMemf string
)

func Start(cf, mf string) {
	log.Info("StartProfile")

	DefaultCPUf = cf
	DefaultMemf = mf

	// if DefaultCPUf != "" {
	//	f, err := os.Create(DefaultCPUf)
	//	if err != nil {
	//		log.Warn("create cpu file error", zap.String("err", err.Error()))
	//	}
	//
	//	if err := pprof.StartCPUProfile(f); err != nil {
	//		log.Warn("start cpu profile error", zap.String("err", err.Error()))
	//	}
	//
	//	go func() {
	//		time.Sleep(time.Second * 30)
	//		pprof.StopCPUProfile()
	//	}()
	//
	// }

	go WebListen("")

	go func() {
		for {
			checkMem()
			time.Sleep(1 * time.Second)
		}
	}()
}

//http://xxx.xxx.xxx.xxx:6060/debug/pprof
func WebListen(port string) {
	if port == "" {
		port = "6060"
	}

	_ = http.ListenAndServe(":"+port, nil)
}

func Stop() {
	if DefaultCPUf != "" {
		pprof.StopCPUProfile()
	}
}

func checkMem() {
	if DefaultMemf != "" {
		f, err := os.Create(DefaultMemf)
		if err != nil {
			log.Warn("create mem file error", zap.String("err", err.Error()))
		}

		runtime.GC() // get up-to-date statistics

		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Warn("write mem file error", zap.String("err", err.Error()))
		}

		f.Close()
	}
}

package plmxs

import (
	"emt/util/timer"
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
)

const (
	nb1024 = 1024
	nb6    = 6
)

func (s *PrometheusMonitor) system() {
	timer.NewTicker(time.Minute/nb6, func() {
		s.getSys()
		s.refreshRequestsGauge()
	}) // go getCpuInfo()
}

func (s *PrometheusMonitor) getSys() {
	m := &runtime.MemStats{}
	runtime.ReadMemStats(m)

	mem, err := strconv.ParseFloat(fmt.Sprintf("%.2f", float64(m.Sys)/float64(nb1024*nb1024)), 64)
	if err != nil {
		fmt.Println("err = ", err)

		return
	}

	s.MemoryUseGauge.With(prometheus.Labels{"micro_name": s.ServiceName}).Set(mem)
	s.MemoryPercent.With(prometheus.Labels{"micro_name": s.ServiceName}).Set(GetMemPercent())
	s.CPUPercent.With(prometheus.Labels{"micro_name": s.ServiceName}).Set(GetCPUPercent())
}

/*
func getCpuInfo() {
	cpuInfos, err := cpu.Info()
	if err != nil {
		fmt.Printf("get cpu info failed, err:%v", err)
	}
	for _, ci := range cpuInfos {
		fmt.Println(ci)
	}
	// CPU使用率
	for {
		percent, _ := cpu.Percent(time.Second, false)
		fmt.Printf("cpu percent:%v\n", percent)
	}
}
*/

func GetCPUPercent() float64 {
	percent, _ := cpu.Percent(time.Second, false)

	return percent[0]
}

func GetMemPercent() float64 {
	memInfo, _ := mem.VirtualMemory()

	return memInfo.UsedPercent
}

func GetDiskPercent() float64 {
	parts, _ := disk.Partitions(true)
	diskInfo, _ := disk.Usage(parts[0].Mountpoint)

	return diskInfo.UsedPercent
}

func (s *PrometheusMonitor) refreshRequestsGauge() {
	s.Lock()
	defer s.Unlock()

	for k, v := range s.handlerRequestsNum {
		s.APIRequestsGauge.With(prometheus.Labels{"handler": k, "micro_name": s.ServiceName}).Set(v)
		s.handlerRequestsNum[k] = 0
	}
}

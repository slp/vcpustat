/*
 * vcpustat
 * Copyright (C) 2017 Sergio Lopez <slp@sinrega.org> 
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"strconv"
	"syscall"
	"time"
)

const QEMUACCT_PATTERN = "/sys/fs/cgroup/cpuacct/machine.slice/machine-qemu*.scope"
const VERSION = "0.2"

type machineToVcpuInfo map[string]map[string]VcpuInfo

type VcpuInfo struct {
	Pid      int64
	CpuUsage int64
}

func triggerPanic() {
	sysrq, err := os.OpenFile("/proc/sysrq-trigger", os.O_WRONLY, 0)
	if err != nil {
		log.Printf("couldn't trigger a panic: %v\n", err)
		return
	}
	defer sysrq.Close()

	sysrq.Write([]byte("c\n"))
}

func getVcpusInfo(machine string) (map[string]VcpuInfo) {
	vcpus, err := filepath.Glob(machine + "/vcpu*")
	if err != nil {
		log.Fatal(err)
	}

	vcpuInfo := make(map[string]VcpuInfo)

	for _, vcpu := range vcpus {
		name := path.Base(vcpu)

		pidBytes, err := ioutil.ReadFile(vcpu + "/tasks")
		if err != nil {
			log.Println(err)
			continue
		}

		usageBytes, err := ioutil.ReadFile(vcpu + "/cpuacct.usage")
		if err != nil {
			log.Println(err)
			continue
		}

		pid, err := strconv.ParseInt(strings.Split(string(pidBytes), "\n")[0], 10, 64)
		if err != nil {
			log.Println(err)
			continue
		}

		usage, err := strconv.ParseInt(strings.Split(string(usageBytes), "\n")[0], 10, 64)
		if err != nil {
			log.Println(err)
			continue
		}

		vcpuInfo[name] = VcpuInfo{pid, usage}
	}

	return vcpuInfo
}

func compareVcpuInfo(machinePath string, startVcpuInfo map[string]VcpuInfo, endVcpuInfo map[string]VcpuInfo, printlevel int64, marklevel, paniclevel int64, settleTime int64) {
	machineName := path.Base(machinePath)

	machinePidBytes, err := ioutil.ReadFile(machinePath + "/emulator/tasks")
	if err != nil {
		log.Printf("can't find PID for %s: %v\n", machineName, err)
		return
	}

	machinePid := strings.Split(string(machinePidBytes), "\n")[0]

	pf, err := os.OpenFile("/proc/" + machinePid, os.O_RDONLY, 0)
	if err != nil {
	    log.Printf("can't find proc dir for %s: %v\n", machineName, err)
	    return
	}
	defer pf.Close()

	info, err := pf.Stat()
	if err != nil {
		log.Printf(" can't stat proc dir for %s: %v\n", machineName, err)
		return
	}

	machineSettled := false
	machineTime := int64(time.Since(info.ModTime()).Seconds())
	if machineTime >= settleTime {
		machineSettled = true
	}

	log.Printf("%s (%d seconds, settled=%t):", machineName, machineTime, machineSettled)

	vcpunums := make([]int, 0, len(startVcpuInfo))
	for v := range startVcpuInfo {
		num, err := strconv.Atoi(strings.TrimLeft(v, "vcpu"))
		if err != nil {
			continue
		}
		vcpunums = append(vcpunums, num)
	}

	sort.Ints(vcpunums)

	for vcpunum := range(vcpunums) {
		vcpu := "vcpu" + strconv.Itoa(vcpunum)
		startInfo := startVcpuInfo[vcpu]

		if endInfo, ok := endVcpuInfo[vcpu]; ok {
			if endInfo.Pid != startInfo.Pid {
				log.Printf(" PID for %s changed from %d to %d", vcpu, startInfo.Pid, endInfo.Pid)
				continue
			}

			if endInfo.CpuUsage < startInfo.CpuUsage {
				log.Printf(" %s usage rolled over (start=%d - end=%d), ignoring\n", vcpu, startInfo.CpuUsage, endInfo.CpuUsage)
				continue
			}

			diff := endInfo.CpuUsage - startInfo.CpuUsage
			if machineSettled && paniclevel != -1 && diff <= paniclevel {
				log.Printf(" %7s: %22d (PANIC)\n", vcpu, diff)
				triggerPanic()
			} else if machineSettled && marklevel != -1 && diff <= marklevel {
				log.Printf(" %7s: %22d (WARN)\n", vcpu, diff)
			} else if printlevel == -1 || diff <= printlevel {
				log.Printf(" %7s: %22d\n", vcpu, diff)
			}
		} else {
			log.Printf(" %s disappeared\n", vcpu)
		}
	}
}

func main() {
	intervalPtr := flag.Int("i", 5, "interval in seconds")
	printlevel := flag.Int64("l", -1, "only print CPU usages equal or lower than printlevel")
	marklevel := flag.Int64("m", -1, "put a (WARN) tag on CPU usages equal or lower than marklevel")
	paniclevel := flag.Int64("p", -1, "trigger a Host panic if CPU usage is equal or lower than paniclevel")
	settletime := flag.Int64("s", 120, "ignore marklevel and paniclevel if machine was running for less than settletime seconds")
	logfile := flag.String("f", "", "log to this file instead of stdout")
	version := flag.Bool("v", false, "display version information")
	flag.Parse()

	log.SetFlags(0)

	if *version {
		log.Printf("vcpustat version %s\n", VERSION)
		log.Printf("(C) Sergio Lopez (slp <at> sinrega.org)\n")
		os.Exit(0)
	}

	if *logfile != "" {
		lf, err := os.OpenFile(*logfile, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
		if err != nil {
		    log.Fatal("error opening logfile: ", err)
		}
		defer lf.Close()

		log.SetOutput(lf)
	}

	if *paniclevel != -1 {
		if syscall.Getuid() != 0 {
			log.Fatal("Enabling paniclevel requires running as root")
		}

		plevelBytes, err := ioutil.ReadFile("/proc/sys/kernel/sysrq")
		if err != nil {
			log.Fatal(err)
		}

		plevel, err := strconv.ParseInt(strings.Split(string(plevelBytes), "\n")[0], 10, 32)
		if err != nil {
			log.Fatal(err)
		}

		if plevel != 1 {
			log.Fatal("Enabling paniclevel requires setting '/proc/sys/kernel/sysrq' to '1'")
		}
	}

	for {
		machines, err := filepath.Glob(QEMUACCT_PATTERN)
		if err != nil {
			log.Fatal(err)
		}

		machineMap := make(map[string]map[string]VcpuInfo)

		for _, machine := range machines {
			name := path.Base(machine)

			machineMap[name] = getVcpusInfo(machine)
		}

		time.Sleep(time.Duration(*intervalPtr) * time.Second)

		log.Println(time.Now().Local())
		for _, machine := range machines {
			name := path.Base(machine)

			compareVcpuInfo(machine, machineMap[name], getVcpusInfo(machine), *printlevel, *marklevel, *paniclevel, *settletime)
		}
		log.Println("")
	}
}

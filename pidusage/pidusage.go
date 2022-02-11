package pidusage

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

const (
	statTypePS   = "ps"
	statTypeProc = "proc"
)

// Info will record cpu and memory data
type Info struct {
	Pcpu float64 //  %CPU
	Rss  float64
}

// Stat will store CPU time struct
type Stat struct {
	utime  float64
	stime  float64
	cutime float64
	cstime float64
	start  float64
	rss    float64
	uptime float64
}

type fn func(int) (*Info, error)

var (
	fnMap       map[string]fn
	platform    string
	history     map[int]Stat
	historyLock sync.Mutex
	eol         string
)

// Linux platform
var (
	clkTck   float64 = 100  // default
	pageSize float64 = 4096 // default
)

func init() {
	platform = runtime.GOOS
	if eol = "\n"; strings.Index(platform, "win") == 0 {
		platform = "win"
		eol = "\r\n"
	}
	history = make(map[int]Stat)
	fnMap = make(map[string]fn)
	wrapPS := wrapper(statTypePS)

	fnMap["darwin"] = wrapPS
	fnMap["sunos"] = wrapPS
	fnMap["freebsd"] = wrapPS

	wrapProc := wrapper(statTypeProc)
	fnMap["openbsd"] = wrapProc
	fnMap["aix"] = wrapPS
	fnMap["linux"] = wrapProc
	fnMap["netbsd"] = wrapProc
	fnMap["win"] = wrapper("win")

	if platform == "linux" || platform == "netbsd" || platform == "openbsd" {
		initProc()
	}
}

func initProc() {
	if clkTckStdout, err := exec.Command("getconf", "CLK_TCK").Output(); err == nil {
		clkTck = parseFloat(formatStdOut(clkTckStdout, 0)[0])
	}

	if pageSizeStdout, err := exec.Command("getconf", "PAGESIZE").Output(); err == nil {
		pageSize = parseFloat(formatStdOut(pageSizeStdout, 0)[0])
	}
}

func wrapper(statType string) func(pid int) (*Info, error) {
	return func(pid int) (*Info, error) {
		return stat(pid, statType)
	}
}

func formatStdOut(stdout []byte, userfulIndex int) []string {
	infoArr := strings.Split(string(stdout), eol)[userfulIndex]
	ret := strings.Fields(infoArr)
	return ret
}

func parseFloat(val string) float64 {
	floatVal, _ := strconv.ParseFloat(val, 64)
	return floatVal
}

func statFromPS(pid int) (*Info, error) {
	sysInfo := &Info{}
	args := "-o pcpu,rss -p"
	if platform == "aix" {
		args = "-o pcpu,rssize -p"
	}
	stdout, _ := exec.Command(statTypePS, args, strconv.Itoa(pid)).Output()
	ret := formatStdOut(stdout, 1)
	if len(ret) == 0 {
		return sysInfo, errors.New("Can't find process with this PID: " + strconv.Itoa(pid))
	}
	sysInfo.Pcpu = parseFloat(ret[0])
	sysInfo.Rss = parseFloat(ret[1]) * 1024
	return sysInfo, nil
}

func statFromProc(pid int) (*Info, error) {
	sysInfo := &Info{}
	uptimeFileBytes, err := ioutil.ReadFile(path.Join("/proc", "uptime"))
	if err != nil {
		return nil, err
	}
	uptime := parseFloat(strings.Split(string(uptimeFileBytes), " ")[0])

	procStatFileBytes, err := ioutil.ReadFile(path.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return nil, err
	}
	splitAfter := strings.SplitAfter(string(procStatFileBytes), ")")

	if len(splitAfter) == 0 || len(splitAfter) == 1 {
		return sysInfo, errors.New("Can't find process with this PID: " + strconv.Itoa(pid))
	}
	infos := strings.Split(splitAfter[1], " ")
	st := &Stat{
		utime:  parseFloat(infos[12]),
		stime:  parseFloat(infos[13]),
		cutime: parseFloat(infos[14]),
		cstime: parseFloat(infos[15]),
		start:  parseFloat(infos[20]) / clkTck,
		rss:    parseFloat(infos[22]),
		uptime: uptime,
	}

	_stime := 0.0
	_utime := 0.0

	historyLock.Lock()
	defer historyLock.Unlock()

	_history := history[pid]

	if _history.stime != 0 {
		_stime = _history.stime
	}

	if _history.utime != 0 {
		_utime = _history.utime
	}
	total := st.stime - _stime + st.utime - _utime
	total = total / clkTck

	seconds := st.start - uptime
	if _history.uptime != 0 {
		seconds = uptime - _history.uptime
	}

	seconds = math.Abs(seconds)
	if seconds == 0 {
		seconds = 1
	}

	history[pid] = *st
	sysInfo.Pcpu = (total / seconds) * 100
	sysInfo.Rss = st.rss * pageSize
	return sysInfo, nil
}

func stat(pid int, statType string) (*Info, error) {
	switch statType {
	case statTypePS:
		return statFromPS(pid)
	case statTypeProc:
		return statFromProc(pid)
	default:
		return nil, fmt.Errorf("unsupported OS %s", runtime.GOOS)
	}
}

// GetStat will return current system CPU and memory data
func GetStat(pid int) (*Info, error) {
	sysInfo, err := fnMap[platform](pid)
	return sysInfo, err
}

package proc

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	util "github.com/intel/rmd/utils/bitmap"
	appConf "github.com/intel/rmd/utils/config"
	"golang.org/x/sys/unix"
)

const (
	// CPUInfoPath is the patch to cpuinfo
	CPUInfoPath = "/proc/cpuinfo"
	// MountInfoPath is the mount info path
	MountInfoPath = "/proc/self/mountinfo"
	//Mounts to check the resctrl is mounted in the same path
	Mounts = "/proc/self/mounts"
)

var (
	// ResctrlPath is the patch to resctrl
	ResctrlPath string
	// MbaInfoPath is the MBA path of
	MbaInfoPath string
	// Resctrl mounted with Mba Mbps
	isMbaMbpsEnabled bool
)

// rdt_a, cat_l3, cdp_l3, cqm, cqm_llc, cqm_occup_llc
// cqm_mbm_total, cqm_mbm_local
func parseCPUInfoFile(flag string) (bool, error) {
	f, err := os.Open(CPUInfoPath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		// NOTE: Text() function does not cause any DOS Resource Exhaustion issue
		// since the file being read is a system file (/proc/cpuinfo).
		text := s.Text()
		flags := strings.Split(text, " ")

		for _, f := range flags {
			if f == flag {
				return true, nil
			}
		}
	}
	if err := s.Err(); err != nil {
		return false, err
	}
	return false, nil
}

// IsResctrlMounted Checks if ResCtrl is mounted and if the path is valid
func IsResctrlMounted(flag string) bool {
	f, err := os.Open(Mounts)
	if err != nil {
		return false
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		// NOTE: Text() function does not cause any DOS Resource Exhaustion issue
		// since the file being read is a system file (/proc/cpuinfo).
		text := s.Text()
		flags := strings.Split(text, " ")

		if len(flags) > 3 && flags[2] == "resctrl" {
			if flags[1] == flag {
				return true
			}
			return false
		}
	}
	return false
}

// IsRdtAvailable returns RDT feature available or not
func IsRdtAvailable() (bool, error) {
	return parseCPUInfoFile("rdt_a")
}

// IsCqmAvailable returns CMT feature available or not
func IsCqmAvailable() (bool, error) {
	return parseCPUInfoFile("cqm")
}

// IsCdpAvailable returns CDP feature available or not
func IsCdpAvailable() (bool, error) {
	return parseCPUInfoFile("cdp_l3")
}

// IsMbaAvailable returns MBA feature available or not
var IsMbaAvailable = func() (bool, error) {
	return parseCPUInfoFile("mba")
}

// SetMbaMbpsMode stores MBA mode currently supported by rmd
func SetMbaMbpsMode(flag bool) {
	isMbaMbpsEnabled = flag
}

// GetMbaMbpsMode returns MBA mode currently supported by rmd
func GetMbaMbpsMode() bool {
	return isMbaMbpsEnabled
}

// IsL3CatAvailable returns L3 CAT feature available or not
var IsL3CatAvailable = func() (bool, error) {
	return parseCPUInfoFile("cat_l3")
}

// we can use shell command: "mount -l -t resctrl"
var findMountDir = func(mountdir string) (string, error) {
	f, err := os.Open(MountInfoPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		text := s.Text()
		if strings.Contains(text, mountdir) {
			// http://man7.org/linux/man-pages/man5/proc.5.html
			// text = strings.Replace(text, " - ", " ", -1)
			// fields := strings.Split(text, " ")[4:]
			return text, nil
		}
	}
	return "", fmt.Errorf("Can not found the mount entry: %s", mountdir)
}

// IsEnableRdt returns if RDT is enabled or not
func IsEnableRdt() bool {
	mount, err := findMountDir(ResctrlPath)
	if err != nil {
		return false
	}
	return len(mount) > 0
}

// IsEnableCdp returns if CDP is enabled or not
func IsEnableCdp() bool {
	var flag = "cdp"
	mount, err := findMountDir(ResctrlPath)
	if err != nil {
		return false
	}
	return strings.Contains(mount, flag)
}

// IsEnableCat returns if CAT is enabled or not
func IsEnableCat() bool {
	var flag = "cdp"
	mount, err := findMountDir(ResctrlPath)
	if err != nil {
		return false
	}
	return !strings.Contains(mount, flag) && len(mount) > 0
}

// IsEnableMba returns if MBA is enabled or not
var IsEnableMba = func() (bool, error) {
	_, err := findMountDir(ResctrlPath)
	if err != nil {
		return false, err
	}
	if stat, err := os.Stat(MbaInfoPath); err == nil {
		if stat.IsDir() {
			return true, nil
		}
		return false, nil
	}
	return false, err
}

// Process struct with pid and command line
type Process struct {
	Pid     int
	CmdLine string
}

// ListProcesses returns all process on the host
var ListProcesses = func() map[string]Process {
	processes := make(map[string]Process)
	files, _ := filepath.Glob("/proc/[0-9]*/cmdline")
	for _, file := range files {

		listfs := strings.Split(file, "/")
		if pid, err := strconv.Atoi(listfs[2]); err == nil {

			cmd, _ := ioutil.ReadFile(file)
			cmdString := strings.Join(strings.Split(string(cmd), "\x00"), " ")
			processes[listfs[2]] = Process{pid, cmdString}
		}
	}

	return processes
}

// GetCPUAffinity returns the affinity of a given task id
func GetCPUAffinity(Pid string) (*util.Bitmap, error) {
	// each uint is 64 bits
	// max support 16 * 64 cpus
	var mask [16]uintptr

	pid, err := strconv.Atoi(Pid)
	if err != nil {
		return nil, err
	}

	_, _, ierr := syscall.RawSyscall(
		unix.SYS_SCHED_GETAFFINITY,
		uintptr(pid),
		uintptr(len(mask)*8),
		uintptr(unsafe.Pointer(&mask[0])),
	)

	// util.Bitmap.Bits accept 32 bit int type, need to covert it
	var bits []int
	for _, i := range mask {
		val := uint(i)
		// FIXME: what's the hell, find low 32 bits
		bits = append(bits, int((val<<32)>>32))
		bits = append(bits, int(val>>32))
	}
	if ierr != 0 {
		return nil, ierr
	}
	// this is so hacking to construct a Bitmap,
	// Bitmap shouldn't expose detail to other package at all
	return &util.Bitmap{
		// Need to set correctly and carefully, this is the total cpu
		// number
		Len:  16 * 64,
		Bits: bits,
	}, nil
}

// SetCPUAffinity set a process/thread's CPU affinity
//func SetCPUAffinity(Pid string, affinity []int) error {
func SetCPUAffinity(Pid string, cpus *util.Bitmap) error {
	var mask [16]uintptr

	pid, err := strconv.Atoi(Pid)
	if err != nil {
		return err
	}

	for idx, bit := range cpus.Bits {
		ubit := uintptr(bit)
		mask[idx/2] |= ubit
	}

	_, _, ierr := syscall.RawSyscall(unix.SYS_SCHED_SETAFFINITY, uintptr(pid), uintptr(len(mask)*8), uintptr(unsafe.Pointer(&mask[0])))
	if ierr != 0 {
		return ierr
	}
	return nil
}

// Init does config initial
func Init() error {
	appconf := appConf.NewConfig()
	ResctrlPath = appconf.Def.SysResctrl
	MbaInfoPath = ResctrlPath + "/info/MB"
	//check if resctrl is mounted
	isresctrlenabled := IsResctrlMounted(ResctrlPath)
	if isresctrlenabled != true {
		fmt.Println("Resctrl mount or Mount path failed:", isresctrlenabled)
		os.Exit(1)
	}
	return nil
}

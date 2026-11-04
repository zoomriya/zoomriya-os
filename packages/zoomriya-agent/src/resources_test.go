package main

import (
	"testing"
	"time"
)

func TestResources(t *testing.T) {
	agent, box := newFakeBox(t)
	box.write("proc/stat", "cpu  1000 0 1000 8000 0 0 0 0 0 0\ncpu0 500 0 500 4000 0 0 0 0 0 0\ncpu1 500 0 500 4000 0 0 0 0 0 0\nintr 1\n")
	box.write("proc/meminfo", "MemTotal:       16384000 kB\nMemFree:         1000000 kB\nMemAvailable:    6348000 kB\nBuffers: 1000 kB\nCached: 3000 kB\nSwapTotal: 2000 kB\nSwapFree: 1500 kB\n")
	box.write("proc/loadavg", "0.52 0.40 0.31 2/300 4000\n")
	// The Ethernet port and a container's link, which isn't counted.
	box.write("sys/class/net/enp1s0/type", "1\n")
	box.write("sys/class/net/enp1s0/device/vendor", "0x8086\n")
	box.write("sys/class/net/enp1s0/statistics/rx_bytes", "1000000\n")
	box.write("sys/class/net/enp1s0/statistics/tx_bytes", "500000\n")
	box.write("sys/class/net/ve-photos/type", "1\n")
	box.write("sys/class/net/ve-photos/statistics/rx_bytes", "1\n")
	// A drive, its partition, and a loop device.
	box.write("sys/block/nvme0n1/device/model", "x\n")
	box.write("proc/diskstats", " 259 0 nvme0n1 10 0 2000 5 20 0 4000 9 0 0 0\n 259 1 nvme0n1p1 10 0 2000 5 20 0 4000 9 0 0 0\n 7 0 loop0 1 0 8 0 0 0 0 0 0 0 0\n")
	// A service and an app.
	box.write("sys/fs/cgroup/system.slice/box-desktop-app.service/cpu.stat", "usage_usec 1000000\nuser_usec 1\n")
	box.write("sys/fs/cgroup/system.slice/box-desktop-app.service/memory.current", "20000000\n")
	box.write("sys/fs/cgroup/machine.slice/systemd-nspawn@photos.service/cpu.stat", "usage_usec 5000000\n")
	box.write("sys/fs/cgroup/machine.slice/systemd-nspawn@photos.service/memory.current", "900000000\n")
	// The CPU's temperature, a fan, and a drive's (shown with the drives).
	box.write("sys/class/hwmon/hwmon0/name", "coretemp\n")
	box.write("sys/class/hwmon/hwmon0/temp1_input", "48500\n")
	box.write("sys/class/hwmon/hwmon0/temp1_label", "Package id 0\n")
	box.write("sys/class/hwmon/hwmon1/name", "nct6775\n")
	box.write("sys/class/hwmon/hwmon1/fan2_input", "1200\n")
	box.write("sys/class/hwmon/hwmon2/name", "nvme\n")
	box.write("sys/class/hwmon/hwmon2/temp1_input", "41850\n")

	// Between the two readings, half a second: 100 ticks, 25 of them busy;
	// 50 kB in and 25 kB out; 1 MB read and 512 kB written; Photos used a
	// quarter of a core's half second.
	agent.Sleep = func(time.Duration) {
		box.write("proc/stat", "cpu  1015 0 1010 8075 0 0 0 0 0 0\ncpu0 500 0 500 4000 0 0 0 0 0 0\ncpu1 500 0 500 4000 0 0 0 0 0 0\n")
		box.write("sys/class/net/enp1s0/statistics/rx_bytes", "1050000\n")
		box.write("sys/class/net/enp1s0/statistics/tx_bytes", "525000\n")
		box.write("proc/diskstats", " 259 0 nvme0n1 12 0 3953 5 21 0 5000 9 0 0 0\n 7 0 loop0 9 0 99999 0 0 0 0 0 0 0 0\n")
		box.write("sys/fs/cgroup/machine.slice/systemd-nspawn@photos.service/cpu.stat", "usage_usec 5125000\n")
	}
	resp, err := agent.Handle(Request{Op: "resources"})
	if err != nil {
		t.Fatal(err)
	}
	if resp["cpu_percent"] != 25.0 || resp["cores"] != 2 {
		t.Errorf("cpu %v on %v cores", resp["cpu_percent"], resp["cores"])
	}
	if resp["memory_total_bytes"] != int64(16384000*1024) || resp["memory_used_bytes"] != int64(10036000*1024) ||
		resp["memory_cached_bytes"] != int64(4000*1024) || resp["swap_used_bytes"] != int64(500*1024) {
		t.Errorf("memory %v of %v", resp["memory_used_bytes"], resp["memory_total_bytes"])
	}
	if load := resp["load"].([]float64); len(load) != 3 || load[0] != 0.52 {
		t.Errorf("load = %v", load)
	}
	net := resp["network"].(Response)
	if net["in_bps"] != 100000.0 || net["out_bps"] != 50000.0 || len(net["links"].([]LinkRate)) != 1 {
		t.Errorf("network = %+v", net)
	}
	disks := resp["disks"].(Response)
	if disks["read_bps"] != 1999872.0 || disks["write_bps"] != 1024000.0 || len(disks["drives"].([]DriveRate)) != 1 {
		t.Errorf("disks = %+v", disks)
	}
	services := resp["services"].([]ServiceUse)
	if len(services) != 2 || services[0].Name != "photos" || services[0].Kind != "app" || services[0].CPUPercent != 12.5 ||
		services[1].Name != "Box Desktop" || services[1].MemoryBytes != 20000000 {
		t.Errorf("services = %+v", services)
	}
	temps, fans := resp["temperatures"].([]Sensor), resp["fans"].([]Sensor)
	if len(temps) != 1 || temps[0].Celsius != 48.5 || temps[0].Label != "Package id 0" || len(fans) != 1 || fans[0].RPM != 1200 {
		t.Errorf("temperatures %+v fans %+v", temps, fans)
	}
}

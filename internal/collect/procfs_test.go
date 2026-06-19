package collect

import "testing"

// Fixtures are real lines captured from a Linux 6.12 host.

func TestParseStat(t *testing.T) {
	data := []byte(`cpu  13683719 18023 9324660 730723287 79076 1568319 707215 1065765 0 0
cpu0 3404686 3858 2324655 182694197 19048 392735 185064 276846 0 0
intr 12345678 0 9 0 0
ctxt 987654321
btime 1700000000
`)
	s := ParseStat(data)
	want := [8]uint64{13683719, 18023, 9324660, 730723287, 79076, 1568319, 707215, 1065765}
	if s.CPU != want {
		t.Errorf("CPU = %v, want %v", s.CPU, want)
	}
	if s.Intr != 12345678 {
		t.Errorf("Intr = %d, want 12345678", s.Intr)
	}
	if s.Ctxt != 987654321 {
		t.Errorf("Ctxt = %d, want 987654321", s.Ctxt)
	}
}

func TestParseDiskstats(t *testing.T) {
	// sda/sdb are whole disks; sda1 a partition; loop0/dm-0/ram0 virtual.
	data := []byte(`   8       0 sda 100 0 2000 0 50 0 4000 0 0 0 0
   8       1 sda1 999 0 9999 0 999 0 9999 0 0 0 0
   8      16 sdb 10 0 200 0 5 0 400 0 0 0 0
   7       0 loop0 1 0 1 0 1 0 1 0 0 0 0
 253       0 dm-0 1 0 1 0 1 0 1 0 0 0 0
   1       0 ram0 1 0 1 0 1 0 1 0 0 0 0
`)
	d := ParseDiskstats(data)
	// Only sda + sdb counted: sectors read 2000+200, written 4000+400,
	// reads 100+10, writes 50+5.
	if d.SectorsRead != 2200 || d.SectorsWritten != 4400 {
		t.Errorf("sectors r/w = %d/%d, want 2200/4400", d.SectorsRead, d.SectorsWritten)
	}
	if d.ReadsDone != 110 || d.WritesDone != 55 {
		t.Errorf("reqs r/w = %d/%d, want 110/55", d.ReadsDone, d.WritesDone)
	}
}

func TestIsWholeDisk(t *testing.T) {
	cases := map[string]bool{
		"sda": true, "sdb": true, "vda": true, "hda": true,
		"nvme0n1": true, "mmcblk0": true,
		"sda1": false, "sdb12": false, "vda2": false,
		"nvme0n1p1": false, "mmcblk0p1": false,
		"loop0": false, "dm-0": false, "ram0": false, "md0": false, "sr0": false, "zram0": false,
	}
	for name, want := range cases {
		if got := isWholeDisk(name); got != want {
			t.Errorf("isWholeDisk(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestParseNetDev(t *testing.T) {
	data := []byte(`Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 521189214  107699    0    0    0     0          0         0 521189214  107699    0    0    0     0       0          0
  eth0: 1000 10 0 0 0 0 0 0 2000 20 0 0 0 0 0 0
 eth0.5: 5 1 0 0 0 0 0 0 5 1 0 0 0 0 0 0
  br0: 300 3 0 0 0 0 0 0 400 4 0 0 0 0 0 0
`)
	n := ParseNetDev(data)
	// lo excluded, eth0.5 (VLAN) excluded; eth0 + br0 summed.
	if n.RecvBytes != 1300 || n.SendBytes != 2400 {
		t.Errorf("recv/send = %d/%d, want 1300/2400", n.RecvBytes, n.SendBytes)
	}
}

func TestParseKV(t *testing.T) {
	data := []byte(`MemTotal:        7862876 kB
MemFree:          627464 kB
Buffers:          277516 kB
pgpgin 120823646
pgpgout 230637152
`)
	m := ParseKV(data)
	if m["MemTotal"] != 7862876 || m["MemFree"] != 627464 || m["Buffers"] != 277516 {
		t.Errorf("meminfo parse wrong: %v", m)
	}
	if m["pgpgin"] != 120823646 || m["pgpgout"] != 230637152 {
		t.Errorf("vmstat parse wrong: %v", m)
	}
}

func TestParseLoadavg(t *testing.T) {
	l := ParseLoadavg([]byte("0.43 0.26 0.18 1/789 12345\n"))
	if l[0] != 0.43 || l[1] != 0.26 || l[2] != 0.18 {
		t.Errorf("loadavg = %v, want [0.43 0.26 0.18]", l)
	}
}

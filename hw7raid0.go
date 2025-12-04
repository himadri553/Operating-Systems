package raid

type RAID0 struct {
    disks []*Disk
}

func NewRAID0(disks []*Disk) *RAID0 {
    return &RAID0{disks}
}

func (r *RAID0) Write(block int, data []byte) error {
    d := r.disks[block % len(r.disks)]
    offset := block / len(r.disks)
    return d.WriteBlock(offset, data)
}

func (r *RAID0) Read(block int) ([]byte, error) {
    d := r.disks[block % len(r.disks)]
    offset := block / len(r.disks)
    return d.ReadBlock(offset)
}

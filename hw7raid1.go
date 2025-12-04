package raid

type RAID1 struct {
    disks []*Disk
}

func NewRAID1(disks []*Disk) *RAID1 {
    return &RAID1{disks}
}

func (r *RAID1) Write(block int, data []byte) error {
    for _, d := range r.disks {
        if err := d.WriteBlock(block, data); err != nil { return err }
    }
    return nil
}

func (r *RAID1) Read(block int) ([]byte, error) {
    return r.disks[0].ReadBlock(block)
}

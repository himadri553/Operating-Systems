package raid

func xorBlocks(a, b []byte) []byte {
    out := make([]byte, len(a))
    for i := range a { out[i] = a[i] ^ b[i] }
    return out
}

type RAID4 struct {
    dataDisks []*Disk
    parity    *Disk
}

func NewRAID4(disks []*Disk) *RAID4 {
    return &RAID4{
        dataDisks: disks[:len(disks)-1],
        parity:    disks[len(disks)-1],
    }
}

func (r *RAID4) Write(block int, data []byte) error {
    stripeDisk := block % len(r.dataDisks)
    offset := block / len(r.dataDisks)

    // Write data
    if err := r.dataDisks[stripeDisk].WriteBlock(offset, data); err != nil {
        return err
    }

    // Recompute stripe parity
    parityVal := make([]byte, BlockSize)
    for i := 0; i < len(r.dataDisks); i++ {
        b, _ := r.dataDisks[i].ReadBlock(offset)
        parityVal = xorBlocks(parityVal, b)
    }
    return r.parity.WriteBlock(offset, parityVal)
}

func (r *RAID4) Read(block int) ([]byte, error) {
    stripeDisk := block % len(r.dataDisks)
    offset := block / len(r.dataDisks)
    return r.dataDisks[stripeDisk].ReadBlock(offset)
}

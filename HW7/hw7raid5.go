package raid

type RAID5 struct {
    disks []*Disk
}

func NewRAID5(disks []*Disk) *RAID5 {
    return &RAID5{disks}
}

func (r *RAID5) Write(block int, data []byte) error {
    n := len(r.disks)
    stripe := block / (n - 1)
    pos := block % (n - 1)

    parityDisk := stripe % n

    dataDiskIndex := 0
    for i := 0; i < n; i++ {
        if i == parityDisk { continue }
        if dataDiskIndex == pos {
            // Write block
            if err := r.disks[i].WriteBlock(stripe, data); err != nil {
                return err
            }
        }
        dataDiskIndex++
    }

    parity := make([]byte, BlockSize)
    for i := 0; i < n; i++ {
        if i == parityDisk { continue }
        b, _ := r.disks[i].ReadBlock(stripe)
        parity = xorBlocks(parity, b)
    }
    return r.disks[parityDisk].WriteBlock(stripe, parity)
}

func (r *RAID5) Read(block int) ([]byte, error) {
    n := len(r.disks)
    stripe := block / (n - 1)
    pos := block % (n - 1)

    parityDisk := stripe % n

    dataDiskIndex := 0
    for i := 0; i < n; i++ {
        if i == parityDisk { continue }
        if dataDiskIndex == pos {
            return r.disks[i].ReadBlock(stripe)
        }
        dataDiskIndex++
    }

    return nil, nil
}

package raid

import (
    "os"
)

const BlockSize = 4096

type Disk struct {
    f *os.File
}

func OpenDisk(filename string) (*Disk, error) {
    f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0666)
    if err != nil { return nil, err }
    return &Disk{f}, nil
}

func (d *Disk) WriteBlock(block int, data []byte) error {
    _, err := d.f.Seek(int64(block*BlockSize), 0)
    if err != nil { return err }
    _, err = d.f.Write(data)
    if err != nil { return err }
    return d.f.Sync()    // fsync required by HW spec
}

func (d *Disk) ReadBlock(block int) ([]byte, error) {
    buf := make([]byte, BlockSize)
    _, err := d.f.Seek(int64(block*BlockSize), 0)
    if err != nil { return nil, err }
    _, err = d.f.Read(buf)
    return buf, err
}

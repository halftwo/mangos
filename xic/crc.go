package xic

import (
	"hash/crc32"
	"hash/crc64"
)

func Crc32Checksum(data []byte) uint32 {
	return crc32.ChecksumIEEE(data)
}

func Crc32Update(crc uint32, data []byte) uint32 {
	return crc32.Update(crc, crc32.IEEETable, data)
}

var crc64Table = crc64.MakeTable(0x95AC9329AC4BC9B5)

func Crc64Checksum(data []byte) uint64 {
	return Crc64Update(0, data)
}

func Crc64Update(crc uint64, data []byte) uint64 {
	// NB: notice the bitwise not operator
	return ^crc64.Update(^crc, crc64Table, data)
}


package main

import (
	"encoding/binary"
	"io/ioutil"
	"net"
	"os"
	"strings"

	log "unknwon.dev/clog/v2"

	"golang.org/x/text/encoding/simplifiedchinese"
)

const (
	// RedirectMode1 [IP][0x01][国家和地区信息的绝对偏移地址]
	RedirectMode1 = 0x01
	// RedirectMode2 [IP][0x02][信息的绝对偏移][...] or [IP][国家][...]
	RedirectMode2 = 0x02
)

func ByteToUInt32(data []byte) uint32 {
	i := uint32(data[0]) & 0xff
	i |= (uint32(data[1]) << 8) & 0xff00
	i |= (uint32(data[2]) << 16) & 0xff0000
	return i
}

// IPDB common ip database
type IPDB struct {
	Data   []byte
	Offset uint32
	IPNum  uint32
}

// setOffset 设置偏移量
func (db *IPDB) SetOffset(offset uint32) {
	db.Offset = offset
}

// readString 获取字符串
func (db *IPDB) ReadString(offset uint32) []byte {
	db.SetOffset(offset)
	data := make([]byte, 0, 30)
	buf := make([]byte, 1)
	for {
		buf = db.ReadData(1)
		if buf[0] == 0 {
			break
		}
		data = append(data, buf[0])
	}
	return data
}

// readData 从文件中读取数据
func (db *IPDB) ReadData(length uint32, offset ...uint32) (rs []byte) {
	if len(offset) > 0 {
		db.SetOffset(offset[0])
	}

	end := db.Offset + length
	dataNum := uint32(len(db.Data))
	if db.Offset > dataNum {
		return nil
	}

	if end > dataNum {
		end = dataNum
	}
	rs = db.Data[db.Offset:end]
	db.Offset = end
	return
}

// readMode 获取偏移值类型
func (db *IPDB) ReadMode(offset uint32) byte {
	mode := db.ReadData(1, offset)
	return mode[0]
}

// ReadUInt24
func (db *IPDB) ReadUInt24() uint32 {
	buf := db.ReadData(3)
	return ByteToUInt32(buf)
}

// readArea 读取区域
func (db *IPDB) ReadArea(offset uint32) []byte {
	mode := db.ReadMode(offset)
	if mode == RedirectMode1 || mode == RedirectMode2 {
		areaOffset := db.ReadUInt24()
		if areaOffset == 0 {
			return []byte("")
		}
		return db.ReadString(areaOffset)
	}
	return db.ReadString(offset)
}

func GetMiddleOffset(start uint32, end uint32, indexLen uint32) uint32 {
	records := ((end - start) / indexLen) >> 1
	return start + records*indexLen
}

type QQwry struct {
	IPDB
}

// NewQQwry new db from path
func NewQQwry(filePath string) QQwry {
	var fileData []byte

	_, err := os.Stat(filePath)
	if err != nil && os.IsNotExist(err) {
		log.Fatal("纯真数据库文件不存在: %v", err)
	} else {
		fileData, err = ioutil.ReadFile(filePath)
		if err != nil {
			log.Fatal("读取纯真数据库文件失败: %v", err)
		}
	}

	buf := fileData[0:8]
	start := binary.LittleEndian.Uint32(buf[:4])
	end := binary.LittleEndian.Uint32(buf[4:])

	return QQwry{
		IPDB: IPDB{
			Data:  fileData,
			IPNum: (end-start)/7 + 1,
		},
	}
}

// Find ip地址查询对应归属地信息
func (db QQwry) Find(ip string) (string, string) {
	if strings.Count(ip, ".") != 3 {
		return "", ""
	}

	ip4 := binary.BigEndian.Uint32(net.ParseIP(ip).To4())

	offset := db.searchIndex(ip4)
	if offset <= 0 {
		return "", ""
	}

	var gbkCountry []byte
	var gbkArea []byte

	mode := db.ReadMode(offset + 4)
	// [IP][0x01][国家和地区信息的绝对偏移地址]
	if mode == RedirectMode1 {
		countryOffset := db.ReadUInt24()
		mode = db.ReadMode(countryOffset)
		if mode == RedirectMode2 {
			c := db.ReadUInt24()
			gbkCountry = db.ReadString(c)
			countryOffset += 4
		} else {
			gbkCountry = db.ReadString(countryOffset)
			countryOffset += uint32(len(gbkCountry) + 1)
		}
		gbkArea = db.ReadArea(countryOffset)
	} else if mode == RedirectMode2 {
		countryOffset := db.ReadUInt24()
		gbkCountry = db.ReadString(countryOffset)
		gbkArea = db.ReadArea(offset + 8)
	} else {
		gbkCountry = db.ReadString(offset + 4)
		gbkArea = db.ReadArea(offset + uint32(5+len(gbkCountry)))
	}

	enc := simplifiedchinese.GBK.NewDecoder()
	country, _ := enc.String(string(gbkCountry))
	area, _ := enc.String(string(gbkArea))

	country = strings.ReplaceAll(country, " CZ88.NET", "")
	area = strings.ReplaceAll(area, " CZ88.NET", "")

	return country, area
}

// searchIndex 查找索引位置
func (db *QQwry) searchIndex(ip uint32) uint32 {
	header := db.ReadData(8, 0)

	start := binary.LittleEndian.Uint32(header[:4])
	end := binary.LittleEndian.Uint32(header[4:])

	buf := make([]byte, 7)
	mid := uint32(0)
	ipUint := uint32(0)

	for {
		mid = GetMiddleOffset(start, end, 7)
		buf = db.ReadData(7, mid)
		ipUint = binary.LittleEndian.Uint32(buf[:4])

		if end-start == 7 {
			offset := ByteToUInt32(buf[4:])
			buf = db.ReadData(7)
			if ip < binary.LittleEndian.Uint32(buf[:4]) {
				return offset
			}
			return 0
		}

		if ipUint > ip {
			end = mid
		} else if ipUint < ip {
			start = mid
		} else if ipUint == ip {
			return ByteToUInt32(buf[4:])
		}
	}
}

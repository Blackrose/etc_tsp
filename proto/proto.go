package proto

import (
	"bytes"
	"fmt"
	"tsp/codec"
	"tsp/utils"
)

const (
	ProtoHeader byte = 0x7e

	TermAck     uint16 = 0x0001
	Register    uint16 = 0x0100
	RegisterAck uint16 = 0x8100
	Unregister  uint16 = 0x0003
	Login       uint16 = 0x0102
	Heartbeat   uint16 = 0x0002
	Gpsinfo     uint16 = 0x0200
	PlatAck     uint16 = 0x8001
	UpdateReq   uint16 = 0x8108
	CtrlReq     uint16 = 0x8105
)

type MutilField struct {
	MsgSum   uint16
	MsgIndex uint16
}

type Header struct {
	MID       uint16
	Attr      uint16
	Version   uint8
	PhoneNum  string
	SeqNum    uint16
	MutilFlag MutilField
}

func (h *Header) IsMutil() bool {
	if ((h.Attr >> 12) & 0x0001) > 0 {
		return true
	}
	return false
}

//BodyLen is a function for get body len
func (h *Header) BodyLen() int {
	return int(h.Attr & 0x03ff)
}

//MakeAttr is gen attr
func MakeAttr(verFlag byte, mut bool, enc byte, lens uint16) uint16 {
	attr := lens & 0x03FF

	if verFlag > 0 {
		attr = attr & 0x4000
	}

	if mut {
		attr = attr & 0x2000
	}

	encMask := (uint16(enc) & 0x0007) << 10
	return attr + encMask
}

//Message is struct for message for jtt808
type Message struct {
	HEADER Header
	BODY   []byte
}

func Version() string {
	return "1.0.0"
}

func Name() string {
	return "jtt808"
}

//Filter is proto Filter api
func Filter(data []byte) ([]Message, int, error) {
	var usedLen int = 0
	msgList := make([]Message, 0)
	var cnt int = 0
	for {
		cnt++
		if cnt > 10 {
			return []Message{}, 0, fmt.Errorf("time too much")
		}
		if usedLen >= len(data) {
			break
		}

		msg, lens, err := filterSigle(data[usedLen:])
		if err != nil {
			usedLen = usedLen + lens
			fmt.Println("err:", err)
			return msgList, usedLen, nil
		}
		usedLen = usedLen + lens
		msgList = append(msgList, msg)
	}
	return msgList, usedLen, nil
}

func filterSigle(data []byte) (Message, int, error) {
	var usedLen int = 0

	startindex := bytes.IndexByte(data, ProtoHeader)
	if startindex >= 0 {
		usedLen = startindex + 1
		endindex := bytes.IndexByte(data[usedLen:], ProtoHeader)
		if endindex >= 0 {
			endindex = endindex + usedLen
			if endindex-(startindex+1) > 15 {
				var msg Message
				msg.HEADER.MID = codec.Bytes2Word(data[usedLen:])
				usedLen = usedLen + 2
				msg.HEADER.Attr = codec.Bytes2Word(data[usedLen:])
				usedLen = usedLen + 2
				msg.HEADER.Version = data[usedLen]
				usedLen = usedLen + 1

				tempPhone := bytes.TrimLeftFunc(data[usedLen:usedLen+10], func(r rune) bool { return r == 0x00 })
				msg.HEADER.PhoneNum = string(tempPhone)
				usedLen = usedLen + 10
				msg.HEADER.SeqNum = codec.Bytes2Word(data[usedLen:])
				usedLen = usedLen + 2

				if msg.HEADER.IsMutil() {
					msg.HEADER.MutilFlag.MsgSum = codec.Bytes2Word(data[usedLen:])
					usedLen = usedLen + 2
					msg.HEADER.MutilFlag.MsgIndex = codec.Bytes2Word(data[usedLen:])
					usedLen = usedLen + 2
				}

				if endindex-1 < usedLen {
					return Message{}, endindex + 1, fmt.Errorf("flag code is too short")
				}

				msg.BODY = make([]byte, endindex-1-usedLen)
				copy(msg.BODY, data[usedLen:endindex-1])
				usedLen = endindex - 1

				rawcs := checkSum(data[startindex+1 : endindex-1])

				if rawcs != data[endindex-1] {
					return Message{}, endindex + 1, fmt.Errorf("cs is not match:%d--%d,index:%d", rawcs, data[endindex-1], endindex-1)
				}

				return msg, endindex + 1, nil
			}

			return Message{}, endindex + 1, fmt.Errorf("flag code is too short")
		}

		return Message{}, startindex, fmt.Errorf("can't find end flag")
	}
	return Message{}, len(data), fmt.Errorf("can't find start flag")
}

func checkSum(data []byte) byte {
	var sum byte = 0
	for _, itemdata := range data {
		sum ^= itemdata
	}
	return sum
}

//Packer is proto Packer api
func Packer(msg Message) []byte {
	data := make([]byte, 0)
	tempbytes := codec.Word2Bytes(msg.HEADER.MID)
	data = append(data, tempbytes...)
	datalen := uint16(len(msg.BODY)) & 0x03FF
	datalen = datalen | 0x4000

	tempbytes = utils.Word2Bytes(datalen)
	data = append(data, tempbytes...)

	data = append(data, msg.HEADER.Version)

	if len(msg.HEADER.PhoneNum) < 10 {
		data = append(data, make([]byte, 10-len(msg.HEADER.PhoneNum))...)
		data = append(data, msg.HEADER.PhoneNum...)
	} else {
		data = append(data, msg.HEADER.PhoneNum[:10]...)
	}

	tempbytes = utils.Word2Bytes(msg.HEADER.SeqNum)
	data = append(data, tempbytes...)

	data = append(data, msg.BODY...)

	csdata := byte(checkSum(data[:]))
	data = append(data, csdata)

	//添加头尾
	var tmpdata []byte = []byte{0x7e}

	for _, item := range data {
		if item == 0x7d {
			tmpdata = append(tmpdata, 0x7d, 0x01)
		} else if item == 0x7e {
			tmpdata = append(tmpdata, 0x7d, 0x02)
		} else {
			tmpdata = append(tmpdata, item)
		}
	}
	tmpdata = append(tmpdata, 0x7e)

	return tmpdata
}
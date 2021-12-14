package protocol

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"net"
	"snowsensor/conf"
	"strings"
	"time"
)

const CONNECT_TIMEOUT = 5
const READ_TIMEOUT = 10

const (
	P_RAW = PROTOCOL(iota)
	P_WENGLOR
)

const (
	WENG_START     = '$'
	WENG_STOP0     = '.'
	WENG_STOP1     = ';'
	WENG_FRAMETYP  = 0
	WENG_CMD_REQ   = 0x0A
	WENG_CMD_GAP   = 0x00
	WENG_CMD_ACK   = 0x01
	WENG_CMD_LASER = 0x09
)

type PROTOCOL int32

type Proto struct {
	p          PROTOCOL
	p_lfdnr    int32
	p_addr     int32
	cfg        conf.Config
	conn       net.Conn
	byteReader *bufio.Reader
}

type ioResult struct {
	length int
	err    error
}

func InitProto(p PROTOCOL, cfg conf.Config) *Proto {
	ch := make(chan net.Conn)

	addressStr := cfg.Host + ":" + cfg.Port
	addr, err := net.ResolveTCPAddr("tcp", addressStr)
	if err != nil {
		glog.Errorf("could not prepare TCP address: %v\n", err)
		return nil
	}

	go func() {
		conn, err := net.DialTCP("tcp", nil, addr)
		if err != nil {
			glog.Errorf("connection to sensor at %v failed: %v\n", addressStr, err)
			ch <- nil
		}
		ch <- conn
	}()

	proto := &Proto{
		p:       p,
		p_lfdnr: 1,
		p_addr:  0,
		cfg:     cfg,
	}

	select {
	case proto.conn = <-ch:
		if proto.conn == nil {
			return nil // connection failed
		}
		proto.byteReader = bufio.NewReader(proto.conn)
		glog.Infof("connection to sensor at %v established\n", addressStr)
	case <-time.After(CONNECT_TIMEOUT * time.Second):
		glog.Errorf("connection attempt to %v timed out", addressStr)
		return nil
	}

	return proto
}

func (p *Proto) GetMeasurement() (float32, int64, error) {
	if p.p != P_WENGLOR {
		return 0, 0, errors.New("unsupported sensor protocol")
	}

	if err := p.setLaser(true); err != nil {
		return 0, 0, fmt.Errorf("turn laser on failed: %v", err)
	}

	time.Sleep(time.Duration(p.cfg.Warmup_ms) * time.Millisecond)

	var height float32
	for i := int32(0); i < p.cfg.Retry; i++ {
		hRaw, err := p.doGetMeasurement()
		if err != nil {
			glog.Warningf("measurement failed (retrying): %v", err)
			time.Sleep(200 * time.Millisecond)
			continue
		}
		height = (p.cfg.Zeroline-float32(hRaw))*p.cfg.Scale + p.cfg.Offset + 0.5
		glog.Infof("measurement height = %v", height)
		break
	}

	for i := int32(0); i < p.cfg.Retry; i++ {
		if err := p.setLaser(false); err != nil {
			glog.Warningf("turn laser off failed: %v", err)
			time.Sleep(2 * time.Second)
		} else {
			break
		}
	}

	when := time.Now().Unix()
	return height, when, nil
}

func (p *Proto) doGetMeasurement() (int32, error) {
	n1, err := p.writeMessageWENG([]byte{}, WENG_CMD_REQ, WENG_CMD_GAP, 0, 0, 0, 0, 0)
	if err != nil {
		return 0, err
	}

	if n1 != 0 {
		return 0, fmt.Errorf("%v bytes of unexpected data returned from measurement", n1)
	}

	var cmd0, cmd1, ack byte
	buf := make([]byte, 512)

	n2, err := p.readMessageWENG(buf, &cmd0, &cmd1, &ack)
	if err != nil {
		return 0, fmt.Errorf("read message failed: %v", err)
	}

	glog.Infof("readMessageWENG returned len=%v, cmd0=%1X, cmd1=%1X, ack=%1d", n2, cmd0, cmd1, ack)

	if n2 == 36 && cmd0 == WENG_CMD_REQ && ack == 0x01 {
		pval := int32(binary.LittleEndian.Uint32(buf[8:]))

		glog.Infof("received measurement: val=%v", pval)
		if pval > 0 {
			return pval, nil
		}
	}

	return 0, errors.New("invalid measurement value: negative")
}

func (p *Proto) Close() {
	p.conn.Close()
}

func (p *Proto) writeMessageWENG(data []byte, cmd0, cmd1, ack byte, p1, p2, p3 int16, p4 int32) (int, error) {
	dlen := len(data)
	hlen := int(dlen + 32)

	var buf = make([]byte, 1024, 1024)

	buf[0] = WENG_START
	buf[1] = WENG_FRAMETYP
	buf[2] = byte(p.p_lfdnr)
	p.p_lfdnr++
	buf[3] = 0
	binary.LittleEndian.PutUint16(buf[4:], uint16(hlen))
	// buf[6] .. buf[7]
	binary.LittleEndian.PutUint16(buf[6:], 0)
	buf[6] = ack
	// buf[8] .. buf[11]
	binary.LittleEndian.PutUint32(buf[8:], uint32(p.p_addr))
	buf[12] = cmd0
	buf[13] = cmd1
	binary.LittleEndian.PutUint16(buf[14:], uint16(p1))
	binary.LittleEndian.PutUint16(buf[16:], uint16(p2))
	binary.LittleEndian.PutUint16(buf[18:], uint16(p3))
	binary.LittleEndian.PutUint32(buf[20:], uint32(p4))
	binary.LittleEndian.PutUint32(buf[24:], uint32(dlen))
	pend := 28 + len(data)
	n := copy(buf[28:pend], data)
	if n != len(data) {
		return n,
			fmt.Errorf("invalid number of bytes copied from data to packet buffer: %v, expected %v",
				n,
				len(data))
	}
	checksumWENG(&buf[pend], buf[:pend])
	buf[pend+1] = 0
	buf[pend+2] = WENG_STOP0
	buf[pend+3] = WENG_STOP1

	logbin(buf[:hlen], false)

	ch := make(chan ioResult)
	go func() {
		n, err := p.conn.Write(buf[:hlen])
		if err != nil {
			ch <- ioResult{err: fmt.Errorf("failed sending request to sensor: %v", err)}
			return
		} else if n != hlen {
			ch <- ioResult{err: fmt.Errorf("partial / failed write (%v of expected %v bytes)", n, hlen)}
			return
		}
		ch <- ioResult{length: n}
	}()

	select {
	case res := <-ch:
		if res.err == nil {
			return dlen, nil
		}
		return res.length, res.err
	case <-time.After(READ_TIMEOUT * time.Second):
		// FIXME close connection?
		return 0, fmt.Errorf("sensor read timeout after %v seconds", READ_TIMEOUT)
	}

	return 0, errors.New("unknown write error")
}

func checksumWENG(dest *byte, input []byte) {
	var sum byte
	for i := 0; i < len(input); i++ {
		sum ^= input[i]
	}
	*dest = sum
}

func (p *Proto) readMessageWENG(data []byte, cmd0 *byte, cmd1 *byte, ack *byte) (int, error) {
	var sum byte
	buf := make([]byte, 1024+96)
	*ack = 0
	*cmd0 = 0
	*cmd1 = 0

	ch := make(chan ioResult)

	go func() {
		// find start of frame
		startTokenSearchRange := 2048
		for i := 0; buf[0] != WENG_START && i < startTokenSearchRange; i++ {
			b, err := p.byteReader.ReadByte()
			if err != nil {
				e := fmt.Errorf("error reading from sensor (start token): %v", err)
				ch <- ioResult{err: e}
				return
			}
			buf[0] = b
		}

		if buf[0] != WENG_START {
			e := fmt.Errorf("error reading from sensor: start token not found after reading %v bytes", startTokenSearchRange)
			ch <- ioResult{err: e}
			return
		}

		// read until both stop tokens
		for i := 1; i < len(buf); i++ {
			b, err := p.byteReader.ReadByte()
			if err != nil {
				e := fmt.Errorf("error reading from sensor (frame): %v", err)
				ch <- ioResult{err: e}
				return
			}
			buf[i] = b

			if b == WENG_STOP1 && buf[i-1] == WENG_STOP0 {
				length := i + 1
				ch <- ioResult{length: length}
				return
			}
		}

		e := fmt.Errorf("error reading from sensor: stop token not found within %v bytes", len(buf))
		ch <- ioResult{err: e}
	}()

	var length int
	select {
	case res := <-ch:
		if res.err != nil {
			return res.length, res.err
		}
		length = res.length
	case <-time.After(READ_TIMEOUT * time.Second):
		// FIXME close connection?
		e := fmt.Errorf("read timeout after %v seconds", READ_TIMEOUT)
		return 0, e
	}

	if length < 32 {
		e := fmt.Errorf("frame error: len=%v, expected at least 32 bytes", length)
		return 0, e
	}
	buf = buf[:length]

	checksumWENG(&sum, buf[:length-4])
	if sum != buf[length-4] {
		e := fmt.Errorf("checksum error: actual sum: %v, packet claims: %v", sum, buf[length-4])
		return 0, e
	}

	logbin(buf, true)

	length -= 32
	if length > len(data) {
		length = len(data)
	}
	copy(data, buf[28:])
	*ack = buf[6]
	*cmd0 = buf[12]
	*cmd1 = buf[13]

	return length, nil
}

func (p *Proto) setLaser(on bool) error {
	var cmd0, cmd1, ack byte
	var p1 int16 = 1
	onStr := "off"
	if on {
		p1 = 0
		onStr = "on"
	}

	glog.Infof("set laser %v", onStr)

	l, err := p.writeMessageWENG([]byte{}, WENG_CMD_REQ, WENG_CMD_LASER, 0, p1, 0, 0, 0)
	if err == nil && l == 0 {
		// no user data
		buf := make([]byte, 256)
		_, err := p.readMessageWENG(buf, &cmd0, &cmd1, &ack)
		if err == nil && cmd0 == WENG_CMD_REQ && ack != 0 {
			return nil
		} else if err != nil {
			return fmt.Errorf("set laser %v failed: %v", onStr, err)
		}
		return fmt.Errorf("set laser %v failed: unexpected response, cmd0=%2x ack=%2x", onStr, cmd0, ack)
	} else {
		return fmt.Errorf("setLaser %v failed (len=%v, error=%v)", onStr, l, err)
	}
}

func logbin(buf []byte, binput bool) {
	if !glog.V(2) {
		return
	}

	sz := strings.Builder{}

	dir := ">>"
	if binput {
		dir = "<<"
	}
	sz.WriteString(dir + " ")

	for i := 0; i < len(buf); i++ {
		if i%16 == 8 {
			sz.WriteString(" -")
		} else if i > 0 && i%16 == 0 {
			glog.Info(sz.String())

			// start new line
			sz = strings.Builder{}
			sz.WriteString(dir + " ")
		}

		sz.WriteString(fmt.Sprintf(" %02x", buf[i]))
	}

	glog.Info(sz.String())
}

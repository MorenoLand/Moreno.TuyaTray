package lanlink

import (
	"bytes"
	"crypto/aes"
	"crypto/md5"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"strconv"
	"time"
)

const (
	cmdControl    = 0x07
	cmdHeartbeat  = 0x09
	cmdDPQuery    = 0x0a
	cmdControlNew = 0x0d
	prefix55AA    = 0x000055AA
	suffix55AA    = 0x0000AA55
	tcpPort       = "6668"
)

var ErrUnsupported = errors.New("unsupported tuya protocol version")

var zeroTime time.Time

type Dps map[string]any

func pkcs7Pad(b []byte) []byte {
	n := aes.BlockSize - len(b)%aes.BlockSize
	return append(b, bytes.Repeat([]byte{byte(n)}, n)...)
}

func pkcs7Unpad(b []byte) ([]byte, error) {
	if len(b) == 0 || len(b)%aes.BlockSize != 0 {
		return nil, errors.New("bad block size")
	}
	n := int(b[len(b)-1])
	if n == 0 || n > aes.BlockSize || n > len(b) {
		return nil, errors.New("bad padding")
	}
	return b[:len(b)-n], nil
}

func ecbEncrypt(key, plain []byte) ([]byte, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	plain = pkcs7Pad(append([]byte(nil), plain...))
	out := make([]byte, len(plain))
	for i := 0; i < len(plain); i += aes.BlockSize {
		c.Encrypt(out[i:], plain[i:i+aes.BlockSize])
	}
	return out, nil
}

func ecbDecrypt(key, enc []byte) ([]byte, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(enc) == 0 || len(enc)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext not block aligned")
	}
	out := make([]byte, len(enc))
	for i := 0; i < len(enc); i += aes.BlockSize {
		c.Decrypt(out[i:], enc[i:i+aes.BlockSize])
	}
	return pkcs7Unpad(out)
}

type frame struct {
	seq, cmd, retcode uint32
	payload           []byte
}

func buildFrame(seq, cmd uint32, payload []byte) []byte {
	var b bytes.Buffer
	for _, v := range []uint32{prefix55AA, seq, cmd, uint32(len(payload) + 8)} {
		_ = binary.Write(&b, binary.BigEndian, v)
	}
	b.Write(payload)
	_ = binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(b.Bytes()))
	_ = binary.Write(&b, binary.BigEndian, uint32(suffix55AA))
	return b.Bytes()
}

func readFrame(r io.Reader) (frame, error) {
	var hdr [16]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return frame{}, err
	}
	if binary.BigEndian.Uint32(hdr[0:4]) != prefix55AA {
		return frame{}, errors.New("bad frame prefix")
	}
	f := frame{seq: binary.BigEndian.Uint32(hdr[4:8]), cmd: binary.BigEndian.Uint32(hdr[8:12])}
	n := binary.BigEndian.Uint32(hdr[12:16])
	if n < 8 || n > 1<<16 {
		return frame{}, errors.New("bad frame length")
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return frame{}, err
	}
	data := body[:n-8]
	if len(data) >= 4 {
		if rc := binary.BigEndian.Uint32(data[:4]); rc&0xFFFFFF00 == 0 {
			f.retcode, data = rc, data[4:]
		}
	}
	f.payload = data
	return f, nil
}

func (f frame) decodeJSON(key []byte) (map[string]json.RawMessage, error) {
	p := f.payload
	if len(p) == 0 {
		return nil, nil
	}
	if bytes.HasPrefix(p, []byte("3.3")) && len(p) > 15 {
		p = p[15:]
	} else if bytes.HasPrefix(p, []byte("3.1")) {
		return nil, ErrUnsupported
	}
	if len(p) > 0 && p[0] == '{' {
		return unmarshalObj(p)
	}
	plain, err := ecbDecrypt(key, p)
	if err != nil {
		return nil, err
	}
	return unmarshalObj(plain)
}

func unmarshalObj(b []byte) (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	return m, json.Unmarshal(b, &m)
}

type Conn struct {
	ID, IP, Version string
	Key             []byte
	Timeout         time.Duration
}

func NewConn(id, ip, version, localKey string) *Conn {
	return &Conn{ID: id, IP: ip, Version: version, Key: []byte(localKey), Timeout: 1200 * time.Millisecond}
}

func (c *Conn) check() error {
	if c.Version != "3.3" && c.Version != "3.1" && c.Version != "" {
		return fmt.Errorf("%w: %s", ErrUnsupported, c.Version)
	}
	if len(c.Key) != 16 {
		return errors.New("local key must be 16 bytes")
	}
	return nil
}

func (c *Conn) dial() (net.Conn, error) {
	conn, err := (&net.Dialer{Timeout: c.Timeout}).Dial("tcp", net.JoinHostPort(c.IP, tcpPort))
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(c.Timeout + 800*time.Millisecond))
	return conn, nil
}

func (c *Conn) body(extra map[string]any) []byte {
	m := map[string]any{"gwId": c.ID, "devId": c.ID, "uid": c.ID, "t": strconv.FormatInt(time.Now().Unix(), 10)}
	for k, v := range extra {
		m[k] = v
	}
	b, _ := json.Marshal(m)
	return b
}

func (c *Conn) sealed(cmd uint32, body []byte) ([]byte, error) {
	enc, err := ecbEncrypt(c.Key, body)
	if err != nil || cmd == cmdDPQuery || cmd == cmdHeartbeat {
		return enc, err
	}
	return append(append([]byte("3.3"), make([]byte, 12)...), enc...), nil
}

func extractDps(m map[string]json.RawMessage) Dps {
	var d Dps
	raw, ok := m["dps"]
	if !ok || json.Unmarshal(raw, &d) != nil {
		return nil
	}
	return d
}

func (c *Conn) readDps(conn net.Conn) (Dps, error) {
	lastErr := errors.New("no status received")
	for i := 0; i < 6; i++ {
		f, err := readFrame(conn)
		if err != nil {
			if i > 0 {
				return nil, lastErr
			}
			return nil, err
		}
		m, err := f.decodeJSON(c.Key)
		if err != nil {
			lastErr = err
			continue
		}
		if d := extractDps(m); d != nil {
			return d, nil
		}
		if e, ok := m["Error"]; ok {
			lastErr = fmt.Errorf("device: %s", string(e))
		}
	}
	return nil, lastErr
}

func (c *Conn) request(cmd uint32, seq uint32, extra map[string]any) (Dps, error) {
	conn, err := c.dial()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	p, err := c.sealed(cmd, c.body(extra))
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write(buildFrame(seq, cmd, p)); err != nil {
		return nil, err
	}
	return c.readDps(conn)
}

func (c *Conn) Status() (Dps, error) {
	if err := c.check(); err != nil {
		return nil, err
	}
	if d, err := c.request(cmdDPQuery, 1, nil); err == nil {
		return d, nil
	}
	q := Dps{}
	for _, k := range []string{"1", "2", "3", "4", "5", "20", "21", "22", "23", "24", "25", "26"} {
		q[k] = nil
	}
	return c.request(cmdControlNew, 2, map[string]any{"dps": q})
}

func (c *Conn) Set(dps Dps) error {
	if err := c.check(); err != nil {
		return err
	}
	conn, err := c.dial()
	if err != nil {
		return err
	}
	defer conn.Close()
	p, err := c.sealed(cmdControl, c.body(map[string]any{"dps": dps}))
	if err != nil {
		return err
	}
	if _, err := conn.Write(buildFrame(3, cmdControl, p)); err != nil {
		return err
	}
	f, err := readFrame(conn)
	if err != nil {
		return err
	}
	if f.retcode != 0 {
		return fmt.Errorf("device rejected command (code %d)", f.retcode)
	}
	return nil
}

type Found struct{ ID, IP, Version string }

var udpKey = func() []byte { s := md5.Sum([]byte("yGAdlopoPVldABfn")); return s[:] }()

func parseBroadcast(pkt []byte) (Found, bool) {
	f, err := readFrame(bytes.NewReader(pkt))
	if err != nil {
		return Found{}, false
	}
	var obj map[string]any
	if p := f.payload; len(p) > 0 && p[0] == '{' {
		if json.Unmarshal(p, &obj) != nil {
			return Found{}, false
		}
	} else if plain, err := ecbDecrypt(udpKey, p); err != nil || json.Unmarshal(plain, &obj) != nil {
		return Found{}, false
	}
	str := func(k string) string { s, _ := obj[k].(string); return s }
	id := str("gwId")
	if id == "" {
		id = str("devId")
	}
	if id == "" || str("ip") == "" {
		return Found{}, false
	}
	return Found{ID: id, IP: str("ip"), Version: str("version")}, true
}

func Discover(d time.Duration) map[string]Found {
	out := map[string]Found{}
	results := make(chan Found, 64)
	deadline := time.Now().Add(d)
	open := 0
	for _, port := range []string{":6666", ":6667"} {
		pc, err := net.ListenPacket("udp4", port)
		if err != nil {
			continue
		}
		open++
		go func(pc net.PacketConn) {
			defer pc.Close()
			buf := make([]byte, 2048)
			for time.Now().Before(deadline) {
				_ = pc.SetReadDeadline(deadline)
				n, _, err := pc.ReadFrom(buf)
				if err != nil {
					break
				}
				if f, ok := parseBroadcast(buf[:n]); ok {
					results <- f
				}
			}
			results <- Found{}
		}(pc)
	}
	for open > 0 {
		if f := <-results; f.ID == "" {
			open--
		} else {
			out[f.ID] = f
		}
	}
	return out
}

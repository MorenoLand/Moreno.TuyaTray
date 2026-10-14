package tuyacloud

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	loginHost = "https://apigw.iotbing.com"
	ClientID  = "HA_3y9q4ak7g4ephrvke"
	Schema    = "haauthorize"
	nonceSet  = "ABCDEFGHJKMNPQRSTWXYZabcdefhijkmnprstwxyz2345678"
)

type Session struct {
	UserCode     string `json:"user_code"`
	Endpoint     string `json:"endpoint"`
	TerminalID   string `json:"terminal_id"`
	UID          string `json:"uid"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpireAtMs   int64  `json:"expire_at_ms"`
}

func (s Session) Valid() bool { return s.Endpoint != "" && s.RefreshToken != "" }

type StatusItem struct {
	Code  string `json:"code"`
	Value any    `json:"value"`
}

type Device struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	LocalKey    string       `json:"local_key"`
	Category    string       `json:"category"`
	ProductID   string       `json:"product_id"`
	ProductName string       `json:"product_name"`
	UUID        string       `json:"uuid"`
	IP          string       `json:"ip"`
	Online      bool         `json:"online"`
	Status      []StatusItem `json:"status"`
	Room        string       `json:"room,omitempty"`
	HomeName    string       `json:"home_name,omitempty"`
}

type Command struct {
	Code  string `json:"code"`
	Value any    `json:"value"`
}

type Home struct{ ID, Name string }

type flexID string

func (f *flexID) UnmarshalJSON(b []byte) error {
	*f = flexID(strings.Trim(string(b), `"`))
	return nil
}

var httpc = &http.Client{Timeout: 20 * time.Second}

type envelope struct {
	Success bool            `json:"success"`
	Code    any             `json:"code"`
	Msg     string          `json:"msg"`
	T       int64           `json:"t"`
	Result  json.RawMessage `json:"result"`
}

func plainCall(method, u string) (envelope, error) {
	req, _ := http.NewRequest(method, u, nil)
	resp, err := httpc.Do(req)
	if err != nil {
		return envelope{}, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var e envelope
	if err := json.Unmarshal(b, &e); err != nil {
		return envelope{}, fmt.Errorf("unexpected response (%d)", resp.StatusCode)
	}
	return e, nil
}

func StartLogin(userCode string) (string, error) {
	e, err := plainCall("POST", fmt.Sprintf("%s/v1.0/m/life/home-assistant/qrcode/tokens?clientid=%s&usercode=%s&schema=%s", loginHost, url.QueryEscape(ClientID), url.QueryEscape(userCode), url.QueryEscape(Schema)))
	if err != nil {
		return "", err
	}
	if !e.Success {
		return "", fmt.Errorf("login rejected: %s", e.Msg)
	}
	var r struct {
		QRCode string `json:"qrcode"`
	}
	if err := json.Unmarshal(e.Result, &r); err != nil || r.QRCode == "" {
		return "", errors.New("no QR token in response")
	}
	return r.QRCode, nil
}

func QRPayload(token string) string { return "tuyaSmart--qrLogin?token=" + token }

func PollLogin(token, userCode string) (Session, bool, error) {
	e, err := plainCall("GET", fmt.Sprintf("%s/v1.0/m/life/home-assistant/qrcode/tokens/%s?clientid=%s&usercode=%s", loginHost, url.PathEscape(token), url.QueryEscape(ClientID), url.QueryEscape(userCode)))
	if err != nil || !e.Success {
		return Session{}, false, err
	}
	var r struct {
		TerminalID   string `json:"terminal_id"`
		Endpoint     string `json:"endpoint"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		UID          string `json:"uid"`
		ExpireTime   int64  `json:"expire_time"`
	}
	if err := json.Unmarshal(e.Result, &r); err != nil {
		return Session{}, false, err
	}
	return Session{UserCode: userCode, Endpoint: r.Endpoint, TerminalID: r.TerminalID, UID: r.UID, AccessToken: r.AccessToken, RefreshToken: r.RefreshToken, ExpireAtMs: e.T + r.ExpireTime*1000}, true, nil
}

type Client struct {
	mu      sync.Mutex
	S       Session
	OnToken func(Session)
}

func NewClient(s Session) *Client { return &Client{S: s} }

func nonce12() []byte {
	b, r := make([]byte, 12), make([]byte, 12)
	_, _ = rand.Read(r)
	for i := range b {
		b[i] = nonceSet[int(r[i])%len(nonceSet)]
	}
	return b
}

func newGCM(secret []byte) (cipher.AEAD, error) {
	blk, err := aes.NewCipher(secret)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(blk)
}

func gcmSeal(plain, secret []byte) (string, error) {
	g, err := newGCM(secret)
	if err != nil {
		return "", err
	}
	n := nonce12()
	return base64.StdEncoding.EncodeToString(n) + base64.StdEncoding.EncodeToString(g.Seal(nil, n, plain, nil)), nil
}

func gcmOpen(b64 string, secret []byte) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(raw) < 28 {
		return nil, errors.New("bad encrypted payload")
	}
	g, err := newGCM(secret)
	if err != nil {
		return nil, err
	}
	return g.Open(nil, raw[:12], raw[12:], nil)
}

func hmacSHA256(key, msg []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(msg)
	return m.Sum(nil)
}

func (c *Client) call(method, path string, params, body map[string]any) (json.RawMessage, error) {
	c.mu.Lock()
	rt := c.S.RefreshToken
	c.mu.Unlock()
	if path != "/v1.0/m/token/"+rt {
		c.refreshIfNeeded()
	}
	c.mu.Lock()
	s := c.S
	c.mu.Unlock()
	rid := make([]byte, 16)
	_, _ = rand.Read(rid)
	rids := fmt.Sprintf("%x-%x-%x-%x-%x", rid[0:4], rid[4:6], rid[6:8], rid[8:10], rid[10:16])
	sum := md5.Sum([]byte(rids + s.RefreshToken))
	hashKey := hex.EncodeToString(sum[:])
	secret := []byte(hex.EncodeToString(hmacSHA256([]byte(rids), []byte(hashKey)))[:16])
	var qEnc, bEnc string
	rawURL := s.Endpoint + path
	if len(params) > 0 {
		j, _ := json.Marshal(params)
		enc, err := gcmSeal(j, secret)
		if err != nil {
			return nil, err
		}
		qEnc, rawURL = enc, rawURL+"?encdata="+url.QueryEscape(enc)
	}
	var bodyReader io.Reader
	if len(body) > 0 {
		j, _ := json.Marshal(body)
		enc, err := gcmSeal(j, secret)
		if err != nil {
			return nil, err
		}
		jb, _ := json.Marshal(map[string]string{"encdata": enc})
		bEnc, bodyReader = enc, bytes.NewReader(jb)
	}
	t := strconv.FormatInt(time.Now().UnixMilli(), 10)
	signStr := "X-appKey=" + ClientID + "||X-requestId=" + rids + "||X-time=" + t
	if s.AccessToken != "" {
		signStr += "||X-token=" + s.AccessToken
	}
	req, err := http.NewRequest(method, rawURL, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-appKey", ClientID)
	req.Header.Set("X-requestId", rids)
	req.Header.Set("X-sid", "")
	req.Header.Set("X-time", t)
	if s.AccessToken != "" {
		req.Header.Set("X-token", s.AccessToken)
	}
	req.Header.Set("X-sign", hex.EncodeToString(hmacSHA256([]byte(hashKey), []byte(signStr+qEnc+bEnc))))
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	var e envelope
	if err := json.Unmarshal(raw, &e); err != nil {
		return nil, err
	}
	if !e.Success {
		return nil, fmt.Errorf("api error %v: %s", e.Code, e.Msg)
	}
	if len(e.Result) == 0 || string(e.Result) == "null" {
		return nil, nil
	}
	var enc string
	if json.Unmarshal(e.Result, &enc) != nil {
		return e.Result, nil
	}
	return gcmOpen(enc, secret)
}

func (c *Client) refreshIfNeeded() {
	c.mu.Lock()
	need, rt := time.Now().UnixMilli()+60_000 > c.S.ExpireAtMs, c.S.RefreshToken
	c.mu.Unlock()
	if !need {
		return
	}
	res, err := c.call("GET", "/v1.0/m/token/"+rt, nil, nil)
	if err != nil || res == nil {
		return
	}
	var r struct {
		ExpireTime   int64  `json:"expireTime"`
		UID          string `json:"uid"`
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}
	if json.Unmarshal(res, &r) != nil || r.AccessToken == "" {
		return
	}
	c.mu.Lock()
	c.S.AccessToken, c.S.RefreshToken, c.S.UID = r.AccessToken, r.RefreshToken, r.UID
	c.S.ExpireAtMs = time.Now().UnixMilli() + r.ExpireTime*1000
	s, cb := c.S, c.OnToken
	c.mu.Unlock()
	if cb != nil {
		cb(s)
	}
}

func (c *Client) Homes() ([]Home, error) {
	res, err := c.call("GET", "/v1.0/m/life/users/homes", nil, nil)
	if err != nil {
		return nil, err
	}
	var list []struct {
		OwnerID flexID `json:"ownerId"`
		Name    string `json:"name"`
	}
	if err := json.Unmarshal(res, &list); err != nil {
		return nil, err
	}
	out := make([]Home, 0, len(list))
	for _, h := range list {
		out = append(out, Home{ID: string(h.OwnerID), Name: h.Name})
	}
	return out, nil
}

func (c *Client) HomeDevices(homeID string) ([]Device, error) {
	res, err := c.call("GET", "/v1.0/m/life/ha/home/devices", map[string]any{"homeId": homeID}, nil)
	if err != nil {
		return nil, err
	}
	var list []Device
	return list, json.Unmarshal(res, &list)
}

func (c *Client) Room(deviceID string) string {
	res, err := c.call("GET", "/v1.0/m/thing/ha/"+deviceID+"/room", nil, nil)
	if err != nil || res == nil {
		return ""
	}
	var r struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(res, &r)
	return r.Name
}

func (c *Client) AllDevices() ([]Device, error) {
	homes, err := c.Homes()
	if err != nil {
		return nil, err
	}
	var out []Device
	for _, h := range homes {
		ds, err := c.HomeDevices(h.ID)
		if err != nil {
			return nil, err
		}
		for i := range ds {
			ds[i].HomeName = h.Name
		}
		out = append(out, ds...)
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)
	for i := range out {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			out[i].Room = c.Room(out[i].ID)
			<-sem
		}(i)
	}
	wg.Wait()
	return out, nil
}

func (c *Client) Send(deviceID string, cmds []Command) error {
	_, err := c.call("POST", "/v1.1/m/thing/"+deviceID+"/commands", nil, map[string]any{"commands": cmds})
	return err
}

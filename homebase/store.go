package homebase

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"

	"moreno/tuyatray/lightshow"
	"moreno/tuyatray/tuyacloud"
)

type Settings struct {
	ShowHouse        bool              `json:"show_house"`
	StartWithWindows bool              `json:"start_with_windows"`
	LocalAPI         bool              `json:"local_api"`
	APIPort          int               `json:"api_port"`
	APIAllow         []string          `json:"api_allow,omitempty"`
	APIAuth          bool              `json:"api_auth"`
	APIUser          string            `json:"api_user,omitempty"`
	APISalt          string            `json:"api_salt,omitempty"`
	APIHash          string            `json:"api_hash,omitempty"`
	Rooms            map[string]string `json:"rooms,omitempty"`
	Slots            map[string]int    `json:"slots,omitempty"`
	Rules            []Rule            `json:"rules,omitempty"`
	Clock24          bool              `json:"clock_24,omitempty"`
	HomePlan         string            `json:"home_plan,omitempty"`
	Icons            map[string]string `json:"icons,omitempty"`
	Show             lightshow.Config  `json:"show"`
}

type Account struct {
	Session tuyacloud.Session  `json:"session"`
	Devices []tuyacloud.Device `json:"devices"`
}

func dir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	d := filepath.Join(base, "TuyaTray")
	_ = os.MkdirAll(d, 0o700)
	return d
}

func ConfigDir() string { return dir() }

func LoadSettings() Settings {
	s := Settings{ShowHouse: true, APIPort: 7480}
	if b, err := os.ReadFile(filepath.Join(dir(), "settings.json")); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}

func SaveSettings(s Settings) error {
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(filepath.Join(dir(), "settings.json"), b, 0o600)
}

func dpapi(in []byte, protect bool) ([]byte, error) {
	if len(in) == 0 {
		return nil, errors.New("empty")
	}
	blob, out := windows.DataBlob{Size: uint32(len(in)), Data: &in[0]}, windows.DataBlob{}
	var err error
	if protect {
		err = windows.CryptProtectData(&blob, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	} else {
		err = windows.CryptUnprotectData(&blob, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	}
	if err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}

func LoadAccount() (Account, bool) {
	var a Account
	b, err := os.ReadFile(filepath.Join(dir(), "account.bin"))
	if err != nil {
		return a, false
	}
	plain, err := dpapi(b, false)
	if err != nil || json.Unmarshal(plain, &a) != nil {
		return a, false
	}
	return a, a.Session.Valid()
}

func SaveAccount(a Account) error {
	b, _ := json.Marshal(a)
	enc, err := dpapi(b, true)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir(), "account.bin"), enc, 0o600)
}

func ForgetAccount() { _ = os.Remove(filepath.Join(dir(), "account.bin")) }

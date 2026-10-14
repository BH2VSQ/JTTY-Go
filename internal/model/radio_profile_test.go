package model

import (
	"encoding/json"
	"testing"
)

func TestAudioSourceMigrationPreservesBothProfiles(t *testing.T) {
	r := DefaultSettings().Radio
	r.Backend = "hamlib-local"
	r.TXAudioSource = "Data"
	r.NET = r.Profile()
	r.NET.TXAudioSource = "Mic"
	r = NormalizeRadio(r)
	if r.TXAudioSource != "Rear" || r.Local.TXAudioSource != "Rear" || r.NET.TXAudioSource != "Front" {
		t.Fatalf("%+v", r)
	}
}

func TestBackendMigrationAndIndependentProfilePersistence(t *testing.T) {
	for _, backend := range []string{"rigctld-net", "hamlib-net-rigctl", "hamlib-rigctld"} {
		r := DefaultSettings().Radio
		r.Backend = backend
		r.RigModelID = 1020
		r.SerialPort = "COM17"
		r.PTTMethod = "DTR"
		r = NormalizeRadio(r)
		expected := "rigctld-net"
		if backend == "hamlib-rigctld" {
			expected = "hamlib-local"
		}
		if r.Backend != expected || r.PTTMethod != "DTR" {
			t.Fatalf("migration %+v", r)
		}
	}
	r := NormalizeRadio(DefaultSettings().Radio)
	r.Backend = "hamlib-local"
	r.RigModelID = 1020
	r.SerialPort = "COM17"
	r.PTTMethod = "RTS"
	r = NormalizeRadio(r)
	local := r.Local
	r.ApplyProfile(r.NET)
	r.Backend = "rigctld-net"
	r.Host = "192.168.1.20"
	r.Port = 4534
	r.PTTMethod = "VOX"
	r = NormalizeRadio(r)
	if r.Local != local {
		t.Fatal("NET save overwrote local profile")
	}
	data, _ := json.Marshal(r)
	var loaded RadioSettings
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	loaded.ApplyProfile(loaded.Local)
	loaded.Backend = "hamlib-local"
	loaded = NormalizeRadio(loaded)
	if loaded.SerialPort != "COM17" || loaded.PTTMethod != "RTS" || loaded.NET.Host != "192.168.1.20" || loaded.NET.Port != 4534 {
		t.Fatalf("profiles %+v", loaded)
	}
	loaded.Backend = "none"
	loaded.Enabled = false
	loaded = NormalizeRadio(loaded)
	if loaded.Local != local {
		t.Fatal("disabled radio lost local profile")
	}
}

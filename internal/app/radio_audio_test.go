package app

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/BH2VSQ/jtty-go/internal/model"
)

func TestNETCATTransmitAudioCommands(t *testing.T) {
	for _, tc := range []struct {
		name, source, pttType, method, want string
	}{
		{"front", "Front", "Rig capable (Mic/Data)", "Rigctl", "T 1"},
		{"rear", "Rear", "Rig capable (Mic/Data)", "Rigctl", "T 3"},
		{"legacy-data", "Data", "Rig capable (Mic/Data)", "CAT", "T 3"},
		{"legacy-mic", "Mic", "Rig capable (Mic/Data)", "Rigctl", "T 1"},
		{"unsupported-keeps-preference", "Rear", "Rig capable", "Rigctl", "T 1"},
		{"serial-ptt", "Rear", "Rig capable (Mic/Data)", "DTR", "T 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			commands := make(chan string, 8)
			var workers sync.WaitGroup
			done := make(chan struct{})
			go func() {
				defer close(done)
				for {
					conn, err := ln.Accept()
					if err != nil {
						return
					}
					workers.Add(1)
					go func() {
						defer workers.Done()
						defer conn.Close()
						scanner := bufio.NewScanner(conn)
						for scanner.Scan() {
							cmd := scanner.Text()
							switch {
							case cmd == "+\\dump_caps":
								fmt.Fprintf(conn, "dump_caps:\nPTT type: %s\nCan set PTT: Y\nCan get Frequency: Y\nRPRT 0\n", tc.pttType)
							case cmd == "f":
								fmt.Fprintln(conn, "14090000")
							case strings.HasPrefix(cmd, "T "):
								commands <- cmd
								fmt.Fprintln(conn, "RPRT 0")
							default:
								fmt.Fprintln(conn, "RPRT -4")
							}
						}
					}()
				}
			}()
			defer func() { ln.Close(); <-done; workers.Wait() }()
			cfg := model.DefaultSettings().Radio
			cfg.Backend = "rigctld-net"
			cfg.Host = "127.0.0.1"
			cfg.Port = ln.Addr().(*net.TCPAddr).Port
			cfg.TXAudioSource = tc.source
			cfg.PTTMethod = tc.method
			r, _, err := New().openConfiguredRadio(cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			select {
			case cmd := <-commands:
				t.Fatalf("connection test transmitted: %s", cmd)
			default:
			}
			if err = r.PTT(context.Background(), true); err != nil {
				t.Fatal(err)
			}
			if cmd := <-commands; cmd != tc.want {
				t.Fatalf("got %s want %s", cmd, tc.want)
			}
			if err = r.PTT(context.Background(), false); err != nil {
				t.Fatal(err)
			}
			if cmd := <-commands; cmd != "T 0" {
				t.Fatal(cmd)
			}
		})
	}
}

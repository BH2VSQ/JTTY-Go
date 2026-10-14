package radio

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

func TestRigctldFrequencyAndPTT(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	type pttCommand struct {
		connection int
		value      string
	}
	pttCommands := make(chan pttCommand, 4)
	var wg sync.WaitGroup
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		nextID := 0
		for {
			c, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			nextID++
			id := nextID
			wg.Add(1)
			go func(c net.Conn, id int) {
				defer wg.Done()
				defer c.Close()
				s := bufio.NewScanner(c)
				for s.Scan() {
					switch s.Text() {
					case "f":
						fmt.Fprintln(c, "14090000")

					case "F 14091000":
						fmt.Fprintln(c, "RPRT 0")
					case "T 1", "T 0":
						pttCommands <- pttCommand{connection: id, value: s.Text()}
						fmt.Fprintln(c, "RPRT 0")
					default:
						fmt.Fprintln(c, "RPRT -4")
					}
				}
			}(c, id)
		}
	}()

	host, port, _ := net.SplitHostPort(ln.Addr().String())
	var p int
	fmt.Sscanf(port, "%d", &p)
	r := NewRigctld(RigctldConfig{Address: host, Port: p})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := r.Open(ctx); err != nil {
		t.Fatal(err)
	}
	freq, err := r.Frequency(ctx)
	if err != nil || freq != 14090000 {
		t.Fatalf("freq=%d err=%v", freq, err)
	}
	if err := r.SetFrequency(ctx, 14091000); err != nil {
		t.Fatal(err)
	}
	if err := r.PTT(ctx, true); err != nil {
		t.Fatal(err)
	}
	if !r.Transmitting() {
		t.Fatal("expected transmitting")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	_ = ln.Close()
	<-acceptDone
	wg.Wait()

	on := <-pttCommands
	off := <-pttCommands
	if on.value != "T 1" || off.value != "T 0" {
		t.Fatalf("expected PTT ON then OFF, got %#v then %#v", on, off)
	}
	if on.connection != off.connection {
		t.Fatalf("PTT ON and OFF used different TCP sessions: %#v %#v", on, off)
	}
}

func TestParseSupportedLevels(t *testing.T) {
	got := parseSupportedLevels([]string{
		"Level: PREAMP ATTENUATOR STRENGTH ALC SWR RFPOWER_METER RFPOWER_METER_WATTS",
		"RPRT 0",
	})
	for _, name := range []string{"STRENGTH", "ALC", "SWR", "RFPOWER_METER", "RFPOWER_METER_WATTS"} {
		if !got[name] {
			t.Fatalf("expected %s to be supported: %#v", name, got)
		}
	}
}

// The default rigctld protocol sends successful getter values without RPRT 0.
// This test protects against waiting for a trailer that will never arrive.
func TestRigctldDefaultProtocolGettersDoNotWaitForRPRT(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	done := make(chan error, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			done <- acceptErr
			return
		}
		defer conn.Close()
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			switch scanner.Text() {
			case "f":
				fmt.Fprintln(conn, "14090000")
			case "m":
				fmt.Fprintln(conn, "USB")
				fmt.Fprintln(conn, "3000")
			default:
				fmt.Fprintln(conn, "RPRT -4")
			}
		}
		done <- scanner.Err()
	}()

	host, portText, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	var port int
	if _, err := fmt.Sscanf(portText, "%d", &port); err != nil {
		t.Fatal(err)
	}
	rig := NewRigctld(RigctldConfig{Address: host, Port: port})
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	if err := rig.Open(ctx); err != nil {
		t.Fatal(err)
	}

	frequency, err := rig.Frequency(ctx)
	if err != nil {
		_ = rig.Close()
		t.Fatalf("frequency GET timed out or failed: %v", err)
	}
	if frequency != 14090000 {
		_ = rig.Close()
		t.Fatalf("frequency=%d, want 14090000", frequency)
	}
	mode, err := rig.Mode(ctx)
	if err != nil {
		_ = rig.Close()
		t.Fatalf("mode GET timed out or failed: %v", err)
	}
	if mode != "USB" {
		_ = rig.Close()
		t.Fatalf("mode=%q, want USB", mode)
	}
	if err := rig.Close(); err != nil {
		t.Fatalf("close rigctld connection: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("fake rigctld server: %v", err)
	}
}

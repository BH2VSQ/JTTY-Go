package radio

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestRigctldFrequencyAndPTT(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	done := make(chan error, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			done <- err
			return
		}
		defer c.Close()
		s := bufio.NewScanner(c)
		for s.Scan() {
			switch s.Text() {
			case "f":
				fmt.Fprintln(c, "14090000")
				fmt.Fprintln(c, "RPRT 0")
			case "F 14091000":
				fmt.Fprintln(c, "RPRT 0")
			case "T 1":
				fmt.Fprintln(c, "RPRT 0")
			case "t":
				fmt.Fprintln(c, "1")
				fmt.Fprintln(c, "RPRT 0")
			default:
				fmt.Fprintln(c, "RPRT -4")
			}
		}
		done <- s.Err()
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
	defer r.Close()
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
	time.Sleep(10 * time.Millisecond)
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	<-done
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

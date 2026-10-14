package radio

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPTTRejectsMalformedResponse(t *testing.T) {
	for _, reply := range []string{"OK", "RPRT nope", "RPRT 0 extra", "RPRT -1", ""} {
		t.Run(reply, func(t *testing.T) {
			client, server := net.Pipe()
			defer client.Close()
			go func() {
				defer server.Close()
				r := bufio.NewReader(server)
				r.ReadString('\n')
				fmt.Fprintln(server, reply)
			}()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if _, err := rigctldPTTCommand(ctx, client, bufio.NewReader(client), 1); err == nil {
				t.Fatal("invalid PTT response accepted")
			}
		})
	}
}

func TestQueryTimeoutDoesNotBlockPTTOffOrConsumeLateReply(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	queryStarted := make(chan struct{}, 1)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				s := bufio.NewScanner(c)
				for s.Scan() {
					if s.Text() == "f" {
						queryStarted <- struct{}{}
						time.Sleep(350 * time.Millisecond)
						fmt.Fprintln(c, "14090000")
					} else if strings.HasPrefix(s.Text(), "T ") {
						fmt.Fprintln(c, "RPRT 0")
					}
				}
			}()
		}
	}()
	host, portText, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portText)
	r := NewRigctld(RigctldConfig{Address: host, Port: port})
	defer r.Close()
	if err = r.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = r.PTT(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		defer cancel()
		_, err := r.Frequency(ctx)
		done <- err
	}()
	<-queryStarted
	started := time.Now()
	if err = r.PTT(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if time.Since(started) > 150*time.Millisecond {
		t.Fatal("status query blocked PTT-OFF")
	}
	if err = <-done; err == nil {
		t.Fatal("query did not time out")
	}
	if _, err = r.Frequency(context.Background()); err == nil {
		t.Fatal("expired session reused")
	}
	if err = r.PTT(context.Background(), true); err == nil {
		t.Fatal("automatically resumed TX after transport fault")
	}
}

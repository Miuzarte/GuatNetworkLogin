package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"time"
)

const (
	TIME_FORMAT      = "2006-01-02 15:04:05"
	TIME_ZONE        = "CST"
	TIME_ZONE_OFFSET = 8 * 60 * 60
)

var timeLoc = time.FixedZone(TIME_ZONE, TIME_ZONE_OFFSET)

var (
	keys = []string{
		"callback",
		"DDDDD", // acc
		"upass", // pw
		"0MKKey", "R1", "R2",
		"R3", // ISP
		"R6", "para", "v6ip", "terminal_type", "lang", "jsVersion", "v", "lang",
	}
	values = []string{
		"dr1003",
		"", // [1] acc
		"", // [2] pw
		"123456", "0", "",
		"", // [6] ISP: 0:校园网, 1:电信, 2:联通, 3:移动
		"0", "00", "", "1", "zh-cn", "4.2.1", "5730", "zh",
	}
)

const (
	PARAM_ACC = 1
	PARAM_PW  = 2
	PARAM_ISP = 6
)

var loginUrl = "http://10.1.2.3/drcom/login?"

var (
	startTime      = time.Now().In(timeLoc)
	triesCount     = 0
	successesCount = 0
)

func init() {
	if len(keys) != len(values) {
		panic("FIXME: keys and values length mismatch")
	}

	if len(os.Args) < 4 || os.Args[1] == "" || os.Args[2] == "" || os.Args[3] == "" {
		fmt.Println("Usage: GuatNetworkLogin <account> <password> <ISP>")
		fmt.Println("ISP: 0:校园网, 1:电信, 2:联通, 3:移动")
		os.Exit(1)
	}
	values[PARAM_ACC] = os.Args[1] // acc
	values[PARAM_PW] = os.Args[2]  // pw
	values[PARAM_ISP] = os.Args[3] // TOOD: figure out which param is ISP

	for i := range len(keys) {
		if i > 0 {
			loginUrl += "&"
		}
		loginUrl += keys[i] + "=" + values[i]
	}

	fmt.Println("loginUrl:")
	fmt.Println(loginUrl)
}

func next630() (till time.Duration) {
	now := time.Now().In(timeLoc)
	next := time.Date(now.Year(), now.Month(), now.Day(), 6, 30, 0, 0, now.Location())
	if now.After(next) {
		next = next.Add(24 * time.Hour)
	}
	till = next.Sub(now)
	fmt.Printf("%s till next 6:30\n", till.Round(time.Second))
	return till
}

func doLogin() {
	// 5min
	const TRIES = 30
	const TRIES_INTERVAL = time.Second * 10

	ts := time.Now().In(timeLoc)

	for tries := TRIES; tries > 0; tries-- {
		triesCount++
		fmt.Println(time.Now().In(timeLoc).Format(TIME_FORMAT))

		resp, err := http.Get(loginUrl)
		if err != nil {
			fmt.Println("failed to get:", err)
			fmt.Println("retrying in", TRIES_INTERVAL)
			time.Sleep(TRIES_INTERVAL)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Println("failed to read body:", err)
			fmt.Println("retrying in", TRIES_INTERVAL)
			time.Sleep(TRIES_INTERVAL)
			continue
		}

		fmt.Println(string(body))
		successesCount++
		return
	}

	fmt.Printf("failed to login after %d tries since %s\n", TRIES, ts.Format(TIME_FORMAT))
}

func main() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	reader := bufio.NewReader(os.Stdin)
	stdinCh := make(chan struct{})
	go func() {
		for {
			_, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return
				}
				fmt.Println("Stdin error:", err)
				continue
			}
			select {
			case stdinCh <- struct{}{}:
			default:
			}
		}
	}()

	timer := time.NewTimer(next630())
	defer timer.Stop()
LOOP:
	for {
		select {
		case <-timer.C:
			doLogin()
			timer.Reset(next630())
		case <-stdinCh:
			doLogin()
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(next630())
		case <-sigChan:
			if triesCount != 0 {
				fmt.Printf("total tries: %d, successes: %d since %s(%s)\n",
					triesCount, successesCount, startTime.Format(TIME_FORMAT), time.Since(startTime).Round(time.Second))
			}
			break LOOP
		}
	}
}

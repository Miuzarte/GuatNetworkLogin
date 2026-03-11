package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"time"
	"unsafe"
)

const (
	TIME_FORMAT      = "2006/01/02 15:04:05"
	TIME_ZONE        = "CST"
	TIME_ZONE_OFFSET = 8 * 60 * 60
)

var timeLoc = time.FixedZone(TIME_ZONE, TIME_ZONE_OFFSET)

var (
	keys = [...]string{
		"callback",
		"DDDDD", // acc
		"upass", // pw
		"0MKKey", "R1", "R2",
		"R3", // ISP
		"R6", "para", "v6ip", "terminal_type", "lang", "jsVersion", "v", "lang",
	}
	values = [...]string{
		"dr1003",
		"", // [1] acc
		"", // [2] pw
		"123456", "0", "",
		"", // [6] ISP: 0:校园网, 1:电信, 2:联通, 3:移动, 4:广电
		"0", "00", "", "1", "zh-cn", "4.2.1", "2250", "zh",
	}
)

const (
	keysLen   = len(keys)
	valuesLen = len(values)
)

func init() {
	if keysLen != valuesLen {
		panic(fmt.Sprintf("[FIXME] keysLen(%d) != valuesLen(%d)", keysLen, valuesLen))
	}
}

const (
	INDEX_PARAM_ACC = 1
	INDEX_PARAM_PW  = 2
	INDEX_PARAM_ISP = 6
)

func init() {
	if len(os.Args) < 4 || os.Args[1] == "" || os.Args[2] == "" || os.Args[3] == "" {
		println("Usage: GuatNetworkLogin <account> <password> <ISP>")
		println("ISP: 0:校园网, 1:电信, 2:联通, 3:移动, 4:广电")
		os.Exit(1)
	}
	values[INDEX_PARAM_ACC] = os.Args[1]
	values[INDEX_PARAM_PW] = os.Args[2]
	values[INDEX_PARAM_ISP] = os.Args[3]
}

const LOGIN_URL = "http://10.1.2.3/drcom/login"

var loginUrl []byte

func toString(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

var (
	startTime           = time.Now().In(timeLoc)
	totalTriesCount     = 0
	totalSuccessesCount = 0
)

const (
	TOTLA_TIMEOUT = time.Minute * 10
)

const (
	DEFAULT_LOGIN_HOUR = 6
	DEFAULT_LOGIN_MIN  = 30
)

var (
	loginHour = DEFAULT_LOGIN_HOUR
	loginMin  = DEFAULT_LOGIN_MIN
	loginSec  = 0
)

func init() {
	// 预分配古法拼接
	expectedLen := len(LOGIN_URL) + keysLen*2 // 1? 14& 15=
	for i := range keysLen {
		expectedLen += len(keys[i]) + len(values[i])
	}

	loginUrl = make([]byte, 0, expectedLen)
	loginUrl = append(loginUrl, LOGIN_URL...)
	for i := range keysLen {
		if i == 0 {
			loginUrl = append(loginUrl, '?')
		} else {
			loginUrl = append(loginUrl, '&')
		}
		loginUrl = append(loginUrl, keys[i]...)
		loginUrl = append(loginUrl, '=')
		loginUrl = append(loginUrl, values[i]...)
	}
	fmt.Printf("loginUrl: %s\n", loginUrl)

	if expectedLen != len(loginUrl) {
		panic(fmt.Sprintf("[FIXME] str slice extended (realloced): %d => %d", expectedLen, len(loginUrl)))
	}
}

func next(hour, min, sec int) (till time.Duration) {
	now := time.Now().In(timeLoc)
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, sec, 0, now.Location())
	if now.After(next) {
		next = next.Add(24 * time.Hour)
	}
	till = next.Sub(now)
	fmt.Print(till.Round(time.Second), " till next ", hour, ":", min, "\n")
	return till
}

var HttpClient = &http.Client{
	Timeout: time.Second * 5, // 内网环境 使用较短的 timeout
}

func doLogin() int {
	defer runtime.GC() // 执行完毕后清理循环中创建的 [*http.Request]

	triesBefore := totalTriesCount

	// 在 [TOTLA_TIMEOUT] 内无限尝试,
	// 通过 request timeout 控制重试间隔
	t := time.Now().In(timeLoc)
	timeEnd := t.Add(TOTLA_TIMEOUT)

	for t.Before(timeEnd) {
		totalTriesCount++
		t = time.Now().In(timeLoc)
		println(t.Format(TIME_FORMAT))

		req, err := http.NewRequest("GET", toString(loginUrl), nil)
		if err != nil {
			panic(err)
		}

		resp, err := HttpClient.Do(req)
		if err != nil {
			fmt.Printf("failed to get: %v\n", err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Printf("failed to read body: %v\n", err)
			continue
		}

		os.Stdout.Write(body)
		os.Stdout.Write([]byte{'\n'})
		totalSuccessesCount++
		return totalTriesCount - triesBefore
	}

	fmt.Printf(
		"failed to login after %d tries since %s, last try at %s\n",
		totalTriesCount-triesBefore,
		t.Format(TIME_FORMAT),
		time.Now().In(timeLoc).Format(TIME_FORMAT),
	)
	return totalTriesCount - triesBefore
}

var sigChan = make(chan os.Signal, 1)

func init() {
	signal.Notify(sigChan, os.Interrupt)
}

var (
	stdinCh = make(chan struct{})
	reader  = bufio.NewReader(os.Stdin)
)

var timer = time.NewTimer(next(DEFAULT_LOGIN_HOUR, DEFAULT_LOGIN_MIN, 0))

func main() {
	defer timer.Stop()

	go func() {
		for {
			_, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return
				}
				fmt.Printf("Stdin error: %v\n", err)
				continue
			}
			select {
			case stdinCh <- struct{}{}:
			default:
			}
		}
	}()

LOOP:
	for {
		select {
		case <-timer.C:
			tries := doLogin()
			tn := time.Now().In(timeLoc)
			loginHour, loginMin = tn.Hour(), tn.Minute()
			loginSec = (tries - 1) * 5
			if loginSec >= 60 {
				loginSec = 0
			}
			timer.Reset(next(loginHour, loginMin, loginSec))
			// 如果在06:31:45登录成功, 则下一次会定时在06:31:45
		case <-stdinCh:
			doLogin()
		case <-sigChan:
			if totalTriesCount != 0 {
				fmt.Printf(
					"total tries: %d, successes: %d, since: %s (%s)\n",
					totalTriesCount,
					totalSuccessesCount,
					startTime.Format(TIME_FORMAT),
					time.Since(startTime).Round(time.Second),
				)
			}
			break LOOP
		}
	}
}

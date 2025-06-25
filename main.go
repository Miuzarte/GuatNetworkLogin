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

const LOGIN_URL = "http://10.1.2.3/drcom/login"

var loginUrl string

var (
	startTime      = time.Now().In(timeLoc)
	triesCount     = 0
	successesCount = 0
)

const (
	DEFAULT_LOGIN_HOUR = 6
	DEFAULT_LOGIN_MIN  = 30
)

func init() {
	if len(keys) != len(values) {
		panic(fmt.Sprintln("FIXME: keys and values length mismatch", len(keys), len(values)))
	}

	if len(os.Args) < 4 || os.Args[1] == "" || os.Args[2] == "" || os.Args[3] == "" {
		fmt.Println("Usage: GuatNetworkLogin <account> <password> <ISP>")
		fmt.Println("ISP: 0:校园网, 1:电信, 2:联通, 3:移动")
		os.Exit(1)
	}
	values[PARAM_ACC] = os.Args[1]
	values[PARAM_PW] = os.Args[2]
	values[PARAM_ISP] = os.Args[3]

	// 预分配古法拼接
	stringLen := len(LOGIN_URL) + len(keys)*2 // 1? 14& 15=
	for i := range len(keys) {
		stringLen += len(keys[i]) + len(values[i])
	}

	str := make([]byte, 0, stringLen)
	str = append(str, LOGIN_URL...)
	for i := range len(keys) {
		if i == 0 {
			str = append(str, '?')
		} else {
			str = append(str, '&')
		}
		str = append(str, keys[i]...)
		str = append(str, '=')
		str = append(str, values[i]...)
	}
	loginUrl = unsafe.String(unsafe.SliceData(str), len(str))
	fmt.Println("loginUrl:")
	fmt.Println(loginUrl)

	if stringLen != len(loginUrl) {
		panic(fmt.Sprintln("FIXME: loginUrl length mismatch", stringLen, len(loginUrl)))
	}
}

func next(hour int, min int) (till time.Duration) {
	now := time.Now().In(timeLoc)
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())
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

func doLogin() {
	defer runtime.GC() // 执行完毕后清理循环中创建的 [*http.Request]

	triesBefore := triesCount

	// 在 5min 内无限尝试, 通过 request timeout 控制重试间隔
	const DURATION = time.Minute * 5
	t := time.Now().In(timeLoc)
	timeEnd := t.Add(DURATION)

	for t.Before(timeEnd) {
		t = time.Now().In(timeLoc)
		time.Sleep(time.Second / 10) // 保险给一个固定间隔
		triesCount++
		fmt.Println(t.Format(TIME_FORMAT))

		req, err := http.NewRequest("GET", loginUrl, nil)
		if err != nil {
			panic("failed to create request: " + err.Error())
		}

		resp, err := HttpClient.Do(req)
		if err != nil {
			fmt.Println("failed to get:", err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Println("failed to read body:", err)
			continue
		}

		fmt.Println(unsafe.String(unsafe.SliceData(body), len(body)))
		successesCount++
		return
	}

	fmt.Println("failed to login after", triesCount-triesBefore,
		"tries since", t.Format(TIME_FORMAT),
		", last try at", time.Now().In(timeLoc).Format(TIME_FORMAT))
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

	timer := time.NewTimer(next(DEFAULT_LOGIN_HOUR, DEFAULT_LOGIN_MIN))
	defer timer.Stop()
LOOP:
	for {
		select {
		case <-timer.C:
			doLogin()
			tn := time.Now().In(timeLoc)
			timer.Reset(next(tn.Hour(), tn.Minute()))
			// 如果在6:31登录成功, 则下一次会定时在6:31
		case <-stdinCh:
			doLogin()
		case <-sigChan:
			if triesCount != 0 {
				fmt.Println("total tries:", triesCount,
					"successes:", successesCount,
					"since", startTime.Format(TIME_FORMAT),
					"(", time.Since(startTime).Round(time.Second), ")")
			}
			break LOOP
		}
	}
}

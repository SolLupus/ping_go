package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

var (
	timeout int64     //超时时间
	num     int       //发送报文数量
	size    int       //缓冲区大小
	stop    bool      //是否需要主动停止
	failpkg int       //丢包数
	sendpkg int       //发包数
	recvpkg int       //接受数
	ps      int   = 4 //发送的数据大小
)

// ICMP报文头
type ICMP struct {
	Type        uint8  //icmp类型
	Code        uint8  //代码
	Checksum    uint16 //校验和
	Identifier  uint16 //标识符
	SequenceNum uint16 //序列号
}

// 命令行参数解析
func ParseArgs() {
	flag.Int64Var(&timeout, "w", 1500, "等待每次回复的超时时间(毫秒)")
	flag.IntVar(&num, "n", 4, "每次发送的请求数")
	flag.IntVar(&size, "l", 32, "缓冲区大小")
	flag.BoolVar(&stop, "t", false, "Ping指定主机,直到停止")
	flag.Parse()
}

// 使用说明
func Usage() {
	argNum := len(os.Args)
	if argNum < 2 {
		fmt.Print(
			`
用法: ping [-t] [-a] [-n count] [-l size] [-f] [-i TTL] [-v TOS]
            [-r count] [-s count] [[-j host-list] | [-k host-list]]
            [-w timeout] [-R] [-S srcaddr] [-c compartment] [-p]
            [-4] [-6] target_name
选项:
    -t             Ping 指定的主机，直到停止。
                   若要查看统计信息并继续操作，请键入 Ctrl+Break；
                   若要停止，请键入 Ctrl+C。
    -a             将地址解析为主机名。
    -n count       要发送的回显请求数。
    -l size        发送缓冲区大小。
    -i TTL         生存时间。
    -v TOS         服务类型(仅适用于 IPv4。该设置已被弃用，
                   对 IP 标头中的服务类型字段没有任何
                   影响)。
    -r count       记录计数跃点的路由(仅适用于 IPv4)。
    -s count       计数跃点的时间戳(仅适用于 IPv4)。
    -w timeout     等待每次回复的超时时间(毫秒)。
    -S srcaddr     要使用的源地址。
    -c compartment 路由隔离舱标识符。
    -p             Ping Hyper-V 网络虚拟化提供程序地址。
    -4             强制使用 IPv4。
    -6             强制使用 IPv6。
		`)
	}
	os.Exit(0)
}

// 校验和生成
func Checksum(data []byte) uint16 {
	var (
		sum    uint32
		length int = len(data)
		index  uint
		csum   uint16
	)
	for length > 1 {
		sum += uint32(data[index])<<8 + uint32(data[index+1])
		index += 2
		length -= 2
	}
	if length == 1 {
		sum += uint32(data[index])
	}
	csum = uint16(sum) + uint16(sum>>16)
	return ^csum
}

func getICMP(seq uint16) ICMP {
	icmp := ICMP{
		Type:        8,
		Code:        0,
		Checksum:    0,
		Identifier:  1,
		SequenceNum: seq,
	}
	var buffer bytes.Buffer
	binary.Write(&buffer, binary.BigEndian, icmp)
	icmp.Checksum = Checksum(buffer.Bytes())
	buffer.Reset()
	return icmp
}

// 发送ICMP探测报文
func sendICMP(icmp ICMP, dst *net.IPAddr) error {
	conn, err := net.DialIP("ip:icmp", nil, dst)
	if err != nil {
		fmt.Printf("无法连接到远程主机:%s.\n", *dst)
		return err
	}
	defer conn.Close()
	var buffer bytes.Buffer
	binary.Write(&buffer, binary.BigEndian, icmp)
	data := make([]byte, 1024)
	binary.Write(&buffer, binary.BigEndian, data[0:ps])
	sendpkg++
	if _, err := conn.Write(buffer.Bytes()); err != nil {
		failpkg++
		log.Fatal(err)
	}
	start := time.Now()
	recv := make([]byte, size)
	conn.SetReadDeadline(time.Now().Add(time.Duration(timeout)))
	recvcnt, err := conn.Read(recv)
	if err != nil {
		failpkg++
		return err
	}
	end := time.Now()
	duration := end.Sub(start).Nanoseconds() / 1e6
	recvpkg++
	fmt.Printf("来自%s的回复:字节=%d 时间=%dms\n", dst.String(), recvcnt, duration)
	return err
}

func main() {
	ParseArgs()
	args := os.Args
	if len(args) < 2 {
		Usage()
	}
	dst, err := net.ResolveIPAddr("ip", args[len(args)-1])
	// fmt.Println(dst)
	if err != nil {
		fmt.Printf("无法解析IP地址 %s\n", dst.String())
		os.Exit(0)
	}
	fmt.Printf("正在Ping %s 具有 %d 字节的数据\n", dst.String(), 28+ps)
	for i := 1; i <= num; i++ {
		icmp := getICMP(uint16(i))
		if err := sendICMP(icmp, dst); err != nil {
			fmt.Printf("错误:%s\n", err)
		}
		time.Sleep(2 * time.Second)
	}
	fmt.Printf("%s的Ping统计信息:\n \t数据包:已发送=%d,已接受=%d,丢失=%d(%f%%丢失)", dst.String(), sendpkg, recvpkg, failpkg, float32(failpkg/(sendpkg*100)))

}

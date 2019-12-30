# 一个TCP长连接设备管理后台工程

## 概述

这个项目最初只是用来进行一个简单的协议测试用的，而且是一个纯粹的后端命令行工程。只是后面想着只有命令行，操作也不太方便，于是便有了添加一个ui的想法。

golang项目要配ui，最佳的还是配一个前端界面。而我本人并非前端出生，js功底太差，所以就想着用vue了。而且作为一个技术人员，ui界面设计也比较差，所以就打算找一个现成的ui框架来用，尝试了ant designer和iview后，决定使用iview来实现。

这个工程采用前后端分离设计：

后端采用golang语言，web框架采用gin，数据库采用postgresql，并使用xorm来简化数据库操作。使用jwt来进行权限控制。日志库采用logrus。

前端基本就是vue的生态环境，主体采用vue，ui采用iview，路由使用vur-router，状态管理使用vuex，js请求使用axios库。token存储在localstorage中，暂时没有存储到vuex中。由于前端需要绘制地图轨迹，所以用到了百度地图api和vue的地图库vue-baidu-map

因为页面为单页面，所以页面路由统一由前端来控制，后端只提供一个根路由用来加载静态数据，然后提供若干api供前端获取数据。

### 页面

目前页面只做了5个

- 登录页面

- 设备管理页面
- 数据页面
- 地图轨迹页面
- 用户管理页面

5个页面均由路由控制，网页默认加载到登录页面。

## 预览

登录界面:  

![login](https://raw.githubusercontent.com/qiuzhiqian/etc_tsp/master/doc/img/login_1.png)

![devices](https://raw.githubusercontent.com/qiuzhiqian/etc_tsp/master/doc/img/devices_1.png)

![monitor](https://raw.githubusercontent.com/qiuzhiqian/etc_tsp/master/doc/img/monitor_1.png)

![map](https://raw.githubusercontent.com/qiuzhiqian/etc_tsp/master/doc/img/map_1.png)

![users](https://raw.githubusercontent.com/qiuzhiqian/etc_tsp/master/doc/img/users_1.png)


[项目地址](https://github.com/qiuzhiqian/etc_tsp)

## 后端模型

```mermaid
graph BT
A(终端A) --> TCPServer
B(终端B) --> TCPServer
C(终端C) --> TCPServer
TCPServer --> Postgresql
Postgresql --> HTTPServer
HTTPServer --> D(ClientA)
HTTPServer --> E(ClientB)
HTTPServer --> F(ClientC)
```



后端需要设计两个服务器，一个TCP，一个HTTP。TCP主要处理与终端的长连接交互，一个TCP连接对应一台终端设备，终端设备唯一标识使用IMEI。HTTP处理与前端的交互，前端需要获取所有可用的终端设备列表，向指定的终端发送命令。所以，为了方便从ip找到对应终端，然后从对应终端找到对应的conn，我们就需要维护一个map：

```go
type Terminal struct {
	authkey   string
	imei      string
	iccid     string
	vin       string
	tboxver   string
	loginTime time.Time
	seqNum    uint16
	phoneNum  string
	Conn      net.Conn
}

var connManger map[string]*Terminal
```

至于为什么要定义成指针的形式，是因为定义成指针后我们可以直接修改map中元素结构体中对应的变量，而不需要重新定义一个元素再赋值。

```go
var connManager map[string]*Terminal
connManager = make(map[string]*Terminal)
connManager["127.0.0.1:11000"]=&Terminal{}
connManager["127.0.0.1:11001"]=&Terminal{}

...

//此处能够轻松的修改对应的phoneNum修改
connManager["127.0.0.1:11001"].phoneNum = "13000000000"
```

相反，下面的这段代码修改起来就要繁琐不少:

```go
var connManager map[string]Terminal
connManager = make(map[string]Terminal)
connManager["127.0.0.1:11000"]=Terminal{}
connManager["127.0.0.1:11001"]=Terminal{}

...
//此处会报错
connManager["127.0.0.1:11001"].phoneNum = "13000000000"

//此处修改需要定义一个临时变量，类似于读改写的模式
term,ok:=connManager["127.0.0.1:11001"]
term.phoneNum = "13000000000"
connManager["127.0.0.1:11001"]=term
```

上面的代码一处会报错

```bash
cannot assign to struct field connManager["127.0.0.1:11001"].phoneNum in map
```

从上面的对比就可以看到，确实是定义成指针更加方便了。

###  TCP的长连接模型

TCP的长连接我们选择这样的一种方式：

- 每个连接分配一个读Goroutine
- 写数据按需分配

如果熟悉socket的话，就知道socket一个服务器创建的基本步骤：

1. 创建socket
2. listen
3. accept

其中accept一般需要轮循调用。golang也基本是同样的流程。

一个简单的TCP服务器示例：

```go
package main

import (
	"fmt"
	"net"
)

type Terminal struct {
	authkey  string
	imei     string
	iccid    string
	vin      string
	tboxver  string
	phoneNum string
	Conn     net.Conn
}

var connManager map[string]*Terminal

func recvConnMsg(conn net.Conn) {
	addr := conn.RemoteAddr()

	var term *Terminal = &Terminal{
		Conn: conn,
	}
	term.Conn = conn
	connManager[addr.String()] = term

	defer func() {
		delete(connManager, addr.String())
		conn.Close()
	}()

	for {
		tempbuf := make([]byte, 1024)
		n, err := conn.Read(tempbuf)

		if err != nil {
			return
		}

		fmt.Println("rcv:", tempbuf[:n])
	}
}

func TCPServer(addr string) {
	connManager = make(map[string]*Terminal)
	listenSock, err := net.Listen("tcp", addr)
	if err != nil {
		return
	}
	defer listenSock.Close()

	for {
		newConn, err := listenSock.Accept()
		if err != nil {
			continue
		}

		go recvConnMsg(newConn)
	}
}

func main() {
	TCPServer(":19903")
}
```

以下是用来测试的客户端代码:

```go
package main

import (
	"fmt"
	"net"
	"time"
)

func main() {
	conn, err := net.Dial("tcp", ":19903")
	if err != nil {
		return
	}

	defer conn.Close()

	var n int = 0
	n, err = conn.Write([]byte("123456"))
	if err != nil {
		return
	}

	fmt.Println("len:", n)

	for {
		time.Sleep(time.Second * 3)
	}
}
```

测试结果:

```bash
$ ./server 
rcv: [49 50 51 52 53 54]
```

## TCP协议整合JTT808协议

前面简单说明了基于golang的net库进行TCP通讯。现在我们需要将现有的协议整合进去。行业内车辆终端一般都是对接交通部的JTT808协议，此处我们要实现的是JTT808-2019版本。

### 消息结构

|标识位|消息头|消息体|校验码|标识位|
| :--: | :--: | :--: | :--: | :--: |
| 0x7e | | | | 0x7e |

标识位应采用0x7e表示，若校验码、消息头以及消息体中出现0x7e及0x7d，则要进行转义处理。转义规则定义如下：

- 先对0x7d进行转义，转换为固定两个字节数据：0x7d 0x01；
- 再对0x7e进行转义，转换为固定两个字节数据：0x7d 0x02。

转义处理过程如下：

发送消息时：先对消息进行封装，然后计算并填充校验码，最后进行转移处理；

接收消息时：先对消息进行转义还原，然后验证校验码，最后解析消息。

示例：发送一包内容为 0x30 0x7e 0x08 0x7d 0x55 的数据包，则经过封装如下：0x7e 0x 30 0x7d 0x02 0x08 0x7d 0x01 0x55 0x7e。

> 注：多字节按照大端顺序传输

### 消息头

| 起始字节 | 字段           | 数据类型 | 描述及要求                                                   |
| -------- | -------------- | -------- | ------------------------------------------------------------ |
| 0        | 消息ID         | WORD     | --                                                           |
| 2        | 消息体属性     | WORD     | 消息体属性格式结构见下表                                     |
| 4        | 协议版本号     | BYTE     | 协议版本号，每次关键修订递增，初始版本为1                    |
| 5        | 终端手机号     | BCD[10]  | 根据安装后终端自身的手机号码转换。手机号不足位的，则在前面补充数字。 |
| 15       | 消息流水号     | WORD     | 按发送顺序从0开始循环累加                                    |
| 17       | 消息包封装选项 | --       | 如果消息体属性中相关标识位确定消息分包处理，则该项有内容，否则无该项 |

消息体属性格式：

| 15   | 14       | 13   | 12~10        | 9~0        |
| ---- | -------- | ---- | ------------ | ---------- |
| 保留 | 版本标识 | 分包 | 数据加密方式 | 消息体长度 |

> 注版本标识位固定为1

加密方式按照如下进行：

- bit10~bit12为数据加密标识位；
- 当此三位为0,标识消息体不加密；
- 当第10位为1,标识消息体经过RSA算法加密；
- 其它位为保留位。

消息分包按照如下要求进行处理：

- 当消息体属性中第13位为1时表示消息体为长消息，进行分包发送处理，具体分包消息由消息包封包项决定；
- 若第13位为0,则消息头中无消息包封装项字段。

消息包封装项内容：

| 起始字节 | 字段       | 数据内容 | 描述及要求           |
| -------- | ---------- | -------- | -------------------- |
| 0        | 消息总包数 | WORD     | 该消息分包后的总包数 |
| 2        | 包序号     | WORD     | 从1开始              |

### 校验码

校验码的计算规则应从消息头首字节开始，同后一字节进行异或操纵直到消息体末字节结束；校
验码长度为一字节。

### 消息体

消息体只需要实现以下几个命令即可：

| 命令         | 消息ID | 说明                       |
| ------------ | ------ | -------------------------- |
| 终端通用应答 | 0x0001 | 终端通用应答               |
| 平台通用应答 | 0x8001 | 平台通用应答               |
| 终端心跳     | 0x0002 | 消息体为空，应答为通用应答 |
| 终端注册     | 0x0100 |                            |
| 终端注册应答 | 0x8100 |                            |
| 终端鉴权     | 0x0102 | 应答为通用应答             |
| 位置信息     | 0x0200 | 应答为通用应答             |

#### 数据格式

终端通用应答：

| 起始字节 | 字段       | 数据内容 | 描述及要求                               |
| -------- | ---------- | -------- | ---------------------------------------- |
| 0        | 应答流水号 | WORD     | 该消息分包后的总包数                     |
| 2        | 应答ID     | WORD     | 对应的平台消息的ID                       |
| 4        | 结果       | BYTE     | 0：成功/确认;1：失败;2消息有误;3：不支持 |

平台通用应答：

| 起始字节 | 字段       | 数据内容 | 描述及要求                                               |
| -------- | ---------- | -------- | -------------------------------------------------------- |
| 0        | 应答流水号 | WORD     | 对应的终端消息流水号                                     |
| 2        | 应答ID     | WORD     | 对应的终端消息的ID                                       |
| 4        | 结果       | BYTE     | 0：成功/确认;1：失败;2消息有误;3：不支持;4：报警处理确认 |

终端注册：

| 起始字节 | 字段       | 数据内容 | 描述及要求                                               |
| -------- | ---------- | -------- | -------------------------------------------------------- |
| 0        | 应答流水号 | WORD     | 对应的终端消息流水号                                     |
| 2        | 应答ID     | WORD     | 对应的终端消息的ID                                       |
| 4        | 结果       | BYTE     | 0：成功/确认;1：失败;2消息有误;3：不支持;4：报警处理确认 |

终端注册应答：

| 起始字节 | 字段       | 数据内容 | 描述及要求                                                   |
| -------- | ---------- | -------- | ------------------------------------------------------------ |
| 0        | 应答流水号 | WORD     | 对应的终端注册消息的流水号                                   |
| 2        | 结果       | BYTE     | 0:成功;1:车辆已被注册;2:数据库中无该车辆;3终端已被注册;4数据库中无该终端 |
| 3        | 鉴权码     | STRING   | 注册结果为成功时，才有该字段                                 |

鉴权：

| 起始字节 | 字段       | 数据内容 | 描述及要求                                            |
| -------- | ---------- | -------- | ----------------------------------------------------- |
| 0        | 鉴权码长度 | BYTE     | ---                                                   |
| n        | 结果       | STRING   | n为鉴权码长度                                         |
| n+1      | 终端IMEI   | BYTE[15] | ---                                                   |
| n+16     | 软件版本号 | BYTE[20] | 厂家自定义版本号，位数不足时，后补0x00，n为鉴权码长度 |

以上就是需要实现的808协议内容，从协议中可以看到。对于协议实现，为了后续拓展方便，我们需要将它分割成两个基本部分：协议解析和协议处理。

## 协议解析

从前面内容我们可以发现，808协议是一个很典型的协议格式：

```
固定字段+变长字段
```

其中固定字段用来检测一个帧格式的完整性和有效性，所以一般会包含一下内容：帧头+变长字段对应的长度+校验。由于这一段的数据格式固定，目的单一，所以处理起来比较简单。

变长字段的长度是由固定字段终端某一个子字段的值决定的，而且这部分的格式比较多变，需要灵活处理。这一字段我们通常称为Body或者Apdu。

我们首先说明变长字段的处理流程。

### Body处理

正因为Body字段格式灵活，所以为了提高代码的复用性和拓展性，我们需要对Body的处理机制进行抽象，提取出一个相对通用的接口出来。

有经验的工程师都知道，一个协议格式处理，无非就是编码和解码。编码我们称之为Marshal，解码我们称之为Unmarshal。对于不同的格式，我们只需要提供不同的Marshal和Unmarshal实现即可。



从前面分析可以知道，我们现在面对的一种格式是类似于Plain的格式，这种格式没有基本的分割符，下面我们就对这种编码来实现Marshal和Unmarshal。我们将这部分逻辑定义为一个codec包

```go
package codec

func Unmarshal(data []byte, v interface{}) (int, error){}
func Marshal(v interface{}) ([]byte, error){}
```

参考官方库解析json的流程，很快我们就想到了用反射来实现这两个功能。

首先我们来分析Unmarshal，我们需要按照v的类型，将data数据按照对应的长度和类型赋值。举个最简单的例子:

```go
func TestSimple(t *testing.T) {
	type Body struct {
		Age1 int8
		Age2 int16
	}

	data := []byte{0x01, 0x02, 0x03}
	pack := Body{}
	i, err := Unmarshal(data, &pack)
	if err != nil {
		t.Errorf("err:%s", err.Error())
	}

	t.Log("len:", i)
	t.Log("pack:", pack)
}
```

```bash
$ go test -v server/codec -run TestSimple
=== RUN   TestSimple
--- PASS: TestSimple (0.00s)
    codec_test.go:20: len: 3
    codec_test.go:21: pack: {1 515}
PASS
ok      server/codec    0.002s
```

对于Body结构体，第一个字段是int8，占用一个字节，所以分配的值是0x01。第二个字段是int16，占用两个字节，分配的值是0x02,0x03，然后把这两个字节按照大端格式组合成一个int16就行了。所以结果就是Age1字段为1(0x01)，Age2字段为515(0x0203)

所以处理的关键是，我们要识别出v interface{}的类型，然后计算该类型对应的大小，再将data中对应大小的数据段组合成对应类型值复制给v中的对应字段。

v interface{}的类型多变，可能会涉及到结构体嵌套等，所以会存在递归处理，当然第一步我们需要获取到v的类型：

```go
rv := reflect.ValueOf(v)
switch rv.Kind() {
    case reflect.Int8:
		//
	case reflect.Uint8:
		//
	case reflect.Int16:
		//
	case reflect.Uint16:
		//
	case reflect.Int32:
		//
	case reflect.Uint32:
		//
	case reflect.Int64:
		//
	case reflect.Uint64:
		//
	case reflect.Float32:
    	//
    case reflect.Float64:
		//
	case reflect.String:
		//
	case reflect.Slice:
		//
	case reflect.Struct:
    	//需要对struct中的每个元素进行解析
}
```

其他的类型都比较好处理，需要说明的是struct类型，首先我们要能够遍历struct中的各个元素，于是我们找到了：

```go
fieldCount := v.NumField()
v.Field(i)
```

NumField()能够获取结构体内部元素个数，然后Field(i)通过指定index就可以获取到指定的元素了。获取到了元素后，我们就需要最这个元素进行再次的Unmarshal，也就是递归。但是此时我们通过v.Field(i)获取到的是reflect.Value类型，而不是interface{}类型了，所以递归的入参我们使用reflect.Value。另外还需要考虑的一个问题是data数据的索引问题，一次调用Unmarshal就会**消耗掉**一定字节的data数据，消耗的长度应该能够被获取到，以方便下一次调用Unmarshal时，能够对入参的data数据索引做正确的设定。因此，Unmarshal函数需要返回一个当前当用后所占用的字节长度。比如int8就是一个字节，struct就是各个字段字节之和。

```go
func Unmarshal(data []byte, v interface{})  (int,error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return 0,fmt.Errorf("error")
	}

	return refUnmarshal(data, reflect.ValueOf(v))
}

func refUnmarshal(data []byte, v reflect.Value)  (int,error) {
	var usedLen int = 0
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Int8:
		usedLen = usedLen + 1
	case reflect.Uint8:
		usedLen = usedLen + 1
	case reflect.Int16:
		if len(data) < 2 {
			return 0, fmt.Errorf("data to short")
		}
		usedLen = usedLen + 2
	case reflect.Uint16:
		if len(data) < 2 {
			return 0, fmt.Errorf("data to short")
		}
		usedLen = usedLen + 2
	case reflect.Int32:
		if len(data) < 4 {
			return 0, fmt.Errorf("data to short")
		}
		usedLen = usedLen + 4
	case reflect.Uint32:
		if len(data) < 4 {
			return 0, fmt.Errorf("data to short")
		}
		usedLen = usedLen + 4
	case reflect.Int64:
		usedLen = usedLen + 8
	case reflect.Uint64:
		usedLen = usedLen + 8
	case reflect.Float32:
		usedLen = usedLen + 4
	case reflect.Float64:
		usedLen = usedLen + 8
	case reflect.String:
		//待处理
	case reflect.Slice:
		//待处理
	case reflect.Struct:
		fieldCount := v.NumField()

		for i := 0; i < fieldCount; i++ {
			l, err := refUnmarshal(data[usedLen:], v.Field(i), v.Type().Field(i), streLen)
			if err != nil {
				return 0, err
			}

			usedLen = usedLen + l
		}
	}
	return usedLen, nil
}
```

解析到这个地方我们发现，我们又遇到了另外的一个问题：我们没有办法单纯的通过类型来获取到string和struct的长度，而且我们还必须处理这两个类型，因为这两个类型在协议处理中是很常见的。既然单纯的通过类型无法判断长度，我们就要借助tag了。我们尝试着在string和slice上设定tag来解决这个问题。但是tag是属于结构体的，只有结构体内部元素才能拥有tag，而且我们不能通过元素本身获取tag，必须通过上层的struct的type才能获取到，所以此时我们入参还要加入一个通过结构体type获取到的对应字段reflect.StructField：

```go
func refUnmarshal(data []byte, v reflect.Value, tag reflect.StructField) (int, error) {
	var usedLen int = 0
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Int8:
		usedLen = usedLen + 1
	case reflect.Uint8:
		usedLen = usedLen + 1
	case reflect.Int16:
		usedLen = usedLen + 2
	case reflect.Uint16:
		usedLen = usedLen + 2
	case reflect.Int32:
		usedLen = usedLen + 4
	case reflect.Uint32:
		usedLen = usedLen + 4
	case reflect.Int64:
		usedLen = usedLen + 8
	case reflect.Uint64:
		usedLen = usedLen + 8
	case reflect.Float32:
		usedLen = usedLen + 4
	case reflect.Float64:
		usedLen = usedLen + 8
	case reflect.String:
		strLen := tag.Tag.Get("len")
		var lens int = 0
		if strLen == "" {
			//
		} else {
			lens64, err := strconv.ParseInt(strLen, 10, 0)
			if err != nil {
				return 0, err
			}

			lens = int(lens64)
		}
		usedLen = usedLen + int(lens)
	case reflect.Slice:
		strLen := tag.Tag.Get("len")
		var lens int = 0
		if strLen == "" {
			//
		} else {
			lens64, err := strconv.ParseInt(strLen, 10, 0)
			if err != nil {
				return 0, err
			}

			lens = int(lens64)
		}

		usedLen = usedLen + int(lens)
	case reflect.Struct:
		fieldCount := v.NumField()

		for i := 0; i < fieldCount; i++ {
			l, err := refUnmarshal(data[usedLen:], v.Field(i), v.Type().Field(i))
			if err != nil {
				return 0, err
			}

			usedLen = usedLen + l
		}
	}
	return usedLen, nil
}
```

这样我们就能过获取到所有的字段对应的长度了，这个很关键。然后我们只需要根据对应的长度，从data中填充对应的数据值即可

```go
func refUnmarshal(data []byte, v reflect.Value, tag reflect.StructField) (int, error) {
	var usedLen int = 0
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Int8:
		v.SetInt(int64(data[0]))
		usedLen = usedLen + 1
	case reflect.Uint8:
		v.SetUint(uint64(data[0]))
		usedLen = usedLen + 1
	case reflect.Int16:
		if len(data) < 2 {
			return 0, fmt.Errorf("data to short")
		}
		v.SetInt(int64(Bytes2Word(data)))
		usedLen = usedLen + 2
	case reflect.Uint16:
		if len(data) < 2 {
			return 0, fmt.Errorf("data to short")
		}
		v.SetUint(uint64(Bytes2Word(data)))
		usedLen = usedLen + 2
	case reflect.Int32:
		if len(data) < 4 {
			return 0, fmt.Errorf("data to short")
		}
		v.SetInt(int64(Bytes2DWord(data)))
		usedLen = usedLen + 4
	case reflect.Uint32:
		if len(data) < 4 {
			return 0, fmt.Errorf("data to short")
		}
		v.SetUint(uint64(Bytes2DWord(data)))
		usedLen = usedLen + 4
	case reflect.Int64:
		v.SetInt(64)
		usedLen = usedLen + 8
	case reflect.Uint64:
		v.SetUint(64)
		usedLen = usedLen + 8
	case reflect.Float32:
		v.SetFloat(32.23)
		usedLen = usedLen + 4
	case reflect.Float64:
		v.SetFloat(64.46)
		usedLen = usedLen + 8
	case reflect.String:
		strLen := tag.Tag.Get("len")
		var lens int = 0
		if strLen == "" {
			//
		} else {
			lens64, err := strconv.ParseInt(strLen, 10, 0)
			if err != nil {
				return 0, err
			}

			lens = int(lens64)
		}

		if len(data) < int(lens) {
			return 0, fmt.Errorf("data to short")
		}

		v.SetString(string(data[:lens]))
		usedLen = usedLen + int(lens)

	case reflect.Slice:
		strLen := tag.Tag.Get("len")
		var lens int = 0
		if strLen == "" {
			//
		} else {
			lens64, err := strconv.ParseInt(strLen, 10, 0)
			if err != nil {
				return 0, err
			}

			lens = int(lens64)
		}

		v.SetBytes(data[:lens])
		usedLen = usedLen + int(lens)
	case reflect.Struct:
		fieldCount := v.NumField()

		for i := 0; i < fieldCount; i++ {
			l, err := refUnmarshal(data[usedLen:], v.Field(i), v.Type().Field(i))
			if err != nil {
				return 0, err
			}

			usedLen = usedLen + l
		}
	}
	return usedLen, nil
}
```

一个基本的Unmarshal函数就完成了。但是这个处理是比较理想的，在实际中可能会存在这样的一种情况：在一个协议中有若干字段，其他的字段都是固定长度，只有一个字段是长度可变的，而这个可变长度的计算是由总体长度-固定长度来计算出来的。在这种情况下，我们需要提前计算出已知字段的固定长度，然后用data长度-固定长度，得到唯一的可变字段的长度。所以我现在要有一个获取这个结构的有效长度的函数。前面的Unmarshal内部已经可以获取到每个字段的长度了，我们只需要把这个函数简单改造一下就行了：

```go
func RequireLen(v interface{}) (int, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return 0, fmt.Errorf("error")
	}

	return refRequireLen(reflect.ValueOf(v), reflect.StructField{})
}

func refRequireLen(v reflect.Value, tag reflect.StructField) (int, error) {
	var usedLen int = 0
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Int8:
		usedLen = usedLen + 1
	case reflect.Uint8:
		usedLen = usedLen + 1
	case reflect.Int16:
		usedLen = usedLen + 2
	case reflect.Uint16:
		usedLen = usedLen + 2
	case reflect.Int32:
		usedLen = usedLen + 4
	case reflect.Uint32:
		usedLen = usedLen + 4
	case reflect.Int64:
		usedLen = usedLen + 8
	case reflect.Uint64:
		usedLen = usedLen + 8
	case reflect.Float32:
		usedLen = usedLen + 4
	case reflect.Float64:
		usedLen = usedLen + 8
	case reflect.String:
		strLen := tag.Tag.Get("len")
		if strLen == "" {
			return 0, nil
		}
		lens, err := strconv.ParseInt(strLen, 10, 0)
		if err != nil {
			return 0, err
		}

		usedLen = usedLen + int(lens)
	case reflect.Slice:
		strLen := tag.Tag.Get("len")
		if strLen == "" {
			return 0, nil
		}
		lens, err := strconv.ParseInt(strLen, 10, 0)
		if err != nil {
			return 0, err
		}

		usedLen = usedLen + int(lens)
	case reflect.Struct:
		fieldCount := v.NumField()

		for i := 0; i < fieldCount; i++ {
			l, err := refRequireLen(v.Field(i), v.Type().Field(i))
			if err != nil {
				return 0, err
			}

			usedLen = usedLen + l
		}
	}
	return usedLen, nil
}
```

这样我们就可以实现一个完整的Unmarshal

```go
func Unmarshal(data []byte, v interface{}) (int, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return 0, fmt.Errorf("error")
	}

	lens, err := RequireLen(v)
	if err != nil {
		return 0, err
	}

	if len(data) < lens {
		return 0, fmt.Errorf("data too short")
	}

	return refUnmarshal(data, reflect.ValueOf(v), reflect.StructField{}, len(data)-lens)
}

func refUnmarshal(data []byte, v reflect.Value, tag reflect.StructField, streLen int) (int, error) {
	var usedLen int = 0
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Int8:
		v.SetInt(int64(data[0]))
		usedLen = usedLen + 1
	case reflect.Uint8:
		v.SetUint(uint64(data[0]))
		usedLen = usedLen + 1
	case reflect.Int16:
		if len(data) < 2 {
			return 0, fmt.Errorf("data to short")
		}
		v.SetInt(int64(Bytes2Word(data)))
		usedLen = usedLen + 2
	case reflect.Uint16:
		if len(data) < 2 {
			return 0, fmt.Errorf("data to short")
		}
		v.SetUint(uint64(Bytes2Word(data)))
		usedLen = usedLen + 2
	case reflect.Int32:
		if len(data) < 4 {
			return 0, fmt.Errorf("data to short")
		}
		v.SetInt(int64(Bytes2DWord(data)))
		usedLen = usedLen + 4
	case reflect.Uint32:
		if len(data) < 4 {
			return 0, fmt.Errorf("data to short")
		}
		v.SetUint(uint64(Bytes2DWord(data)))
		usedLen = usedLen + 4
	case reflect.Int64:
		v.SetInt(64)
		usedLen = usedLen + 8
	case reflect.Uint64:
		v.SetUint(64)
		usedLen = usedLen + 8
	case reflect.Float32:
		v.SetFloat(32.23)
		usedLen = usedLen + 4
	case reflect.Float64:
		v.SetFloat(64.46)
		usedLen = usedLen + 8
	case reflect.String:
		strLen := tag.Tag.Get("len")
		var lens int = 0
		if strLen == "" {
			lens = streLen
		} else {
			lens64, err := strconv.ParseInt(strLen, 10, 0)
			if err != nil {
				return 0, err
			}

			lens = int(lens64)
		}

		if len(data) < int(lens) {
			return 0, fmt.Errorf("data to short")
		}

		v.SetString(string(data[:lens]))
		usedLen = usedLen + int(lens)

	case reflect.Slice:
		strLen := tag.Tag.Get("len")
		var lens int = 0
		if strLen == "" {
			lens = streLen
		} else {
			lens64, err := strconv.ParseInt(strLen, 10, 0)
			if err != nil {
				return 0, err
			}

			lens = int(lens64)
		}

		v.SetBytes(data[:lens])
		usedLen = usedLen + int(lens)
	case reflect.Struct:
		fieldCount := v.NumField()

		for i := 0; i < fieldCount; i++ {
			l, err := refUnmarshal(data[usedLen:], v.Field(i), v.Type().Field(i), streLen)
			if err != nil {
				return 0, err
			}

			usedLen = usedLen + l
		}
	}
	return usedLen, nil
}
```

理解了上面的流程，Marshal就就很好写了，只是复制过程反过来就行了。这其中还有一些小的转换逻辑将字节数组转换成多字节整形：Bytes2Word、Word2Bytes、Bytes2DWord、Dword2Bytes。这类转换都使用大端格式处理。完整代码如下：

```go
package codec

import (
	"fmt"
	"reflect"
	"strconv"
)

func RequireLen(v interface{}) (int, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return 0, fmt.Errorf("error")
	}

	return refRequireLen(reflect.ValueOf(v), reflect.StructField{})
}

func Unmarshal(data []byte, v interface{}) (int, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return 0, fmt.Errorf("error")
	}

	lens, err := RequireLen(v)
	if err != nil {
		return 0, err
	}

	if len(data) < lens {
		return 0, fmt.Errorf("data too short")
	}

	return refUnmarshal(data, reflect.ValueOf(v), reflect.StructField{}, len(data)-lens)
}

func Marshal(v interface{}) ([]byte, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return []byte{}, fmt.Errorf("error")
	}

	return refMarshal(reflect.ValueOf(v), reflect.StructField{})
}

func refRequireLen(v reflect.Value, tag reflect.StructField) (int, error) {
	var usedLen int = 0
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Int8:
		usedLen = usedLen + 1
	case reflect.Uint8:
		usedLen = usedLen + 1
	case reflect.Int16:
		usedLen = usedLen + 2
	case reflect.Uint16:
		usedLen = usedLen + 2
	case reflect.Int32:
		usedLen = usedLen + 4
	case reflect.Uint32:
		usedLen = usedLen + 4
	case reflect.Int64:
		usedLen = usedLen + 8
	case reflect.Uint64:
		usedLen = usedLen + 8
	case reflect.Float32:
		usedLen = usedLen + 4
	case reflect.Float64:
		usedLen = usedLen + 8
	case reflect.String:
		strLen := tag.Tag.Get("len")
		if strLen == "" {
			return 0, nil
		}
		lens, err := strconv.ParseInt(strLen, 10, 0)
		if err != nil {
			return 0, err
		}

		usedLen = usedLen + int(lens)
	case reflect.Slice:
		strLen := tag.Tag.Get("len")
		if strLen == "" {
			return 0, nil
		}
		lens, err := strconv.ParseInt(strLen, 10, 0)
		if err != nil {
			return 0, err
		}

		usedLen = usedLen + int(lens)
	case reflect.Struct:
		fieldCount := v.NumField()

		for i := 0; i < fieldCount; i++ {
			l, err := refRequireLen(v.Field(i), v.Type().Field(i))
			if err != nil {
				return 0, err
			}

			usedLen = usedLen + l
		}
	}
	return usedLen, nil
}

func refUnmarshal(data []byte, v reflect.Value, tag reflect.StructField, streLen int) (int, error) {
	var usedLen int = 0
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Int8:
		v.SetInt(int64(data[0]))
		usedLen = usedLen + 1
	case reflect.Uint8:
		v.SetUint(uint64(data[0]))
		usedLen = usedLen + 1
	case reflect.Int16:
		if len(data) < 2 {
			return 0, fmt.Errorf("data to short")
		}
		v.SetInt(int64(Bytes2Word(data)))
		usedLen = usedLen + 2
	case reflect.Uint16:
		if len(data) < 2 {
			return 0, fmt.Errorf("data to short")
		}
		v.SetUint(uint64(Bytes2Word(data)))
		usedLen = usedLen + 2
	case reflect.Int32:
		if len(data) < 4 {
			return 0, fmt.Errorf("data to short")
		}
		v.SetInt(int64(Bytes2DWord(data)))
		usedLen = usedLen + 4
	case reflect.Uint32:
		if len(data) < 4 {
			return 0, fmt.Errorf("data to short")
		}
		v.SetUint(uint64(Bytes2DWord(data)))
		usedLen = usedLen + 4
	case reflect.Int64:
		v.SetInt(64)
		usedLen = usedLen + 8
	case reflect.Uint64:
		v.SetUint(64)
		usedLen = usedLen + 8
	case reflect.Float32:
		v.SetFloat(32.23)
		usedLen = usedLen + 4
	case reflect.Float64:
		v.SetFloat(64.46)
		usedLen = usedLen + 8
	case reflect.String:
		strLen := tag.Tag.Get("len")
		var lens int = 0
		if strLen == "" {
			lens = streLen
		} else {
			lens64, err := strconv.ParseInt(strLen, 10, 0)
			if err != nil {
				return 0, err
			}

			lens = int(lens64)
		}

		if len(data) < int(lens) {
			return 0, fmt.Errorf("data to short")
		}

		v.SetString(string(data[:lens]))
		usedLen = usedLen + int(lens)

	case reflect.Slice:
		strLen := tag.Tag.Get("len")
		var lens int = 0
		if strLen == "" {
			lens = streLen
		} else {
			lens64, err := strconv.ParseInt(strLen, 10, 0)
			if err != nil {
				return 0, err
			}

			lens = int(lens64)
		}

		v.SetBytes(data[:lens])
		usedLen = usedLen + int(lens)
	case reflect.Struct:
		fieldCount := v.NumField()

		for i := 0; i < fieldCount; i++ {
			l, err := refUnmarshal(data[usedLen:], v.Field(i), v.Type().Field(i), streLen)
			if err != nil {
				return 0, err
			}

			usedLen = usedLen + l
		}
	}
	return usedLen, nil
}

func refMarshal(v reflect.Value, tag reflect.StructField) ([]byte, error) {
	data := make([]byte, 0)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Int8:
		data = append(data, byte(v.Int()))
	case reflect.Uint8:
		data = append(data, byte(v.Uint()))
	case reflect.Int16:
		temp := Word2Bytes(uint16(v.Int()))
		data = append(data, temp...)
	case reflect.Uint16:
		temp := Word2Bytes(uint16(v.Uint()))
		data = append(data, temp...)
	case reflect.Int32:
		temp := Dword2Bytes(uint32(v.Int()))
		data = append(data, temp...)
	case reflect.Uint32:
		temp := Dword2Bytes(uint32(v.Uint()))
		data = append(data, temp...)
	case reflect.String:
		strLen := tag.Tag.Get("len")
		lens, err := strconv.ParseInt(strLen, 10, 0)
		if err != nil {
			return []byte{}, err
		}

		if int(lens) > v.Len() {
			zeroSlice := make([]byte, int(lens)-v.Len())
			data = append(data, zeroSlice...)
		}
		data = append(data, v.String()...)
	case reflect.Slice:
		strLen := tag.Tag.Get("len")
		lens, err := strconv.ParseInt(strLen, 10, 0)
		if err != nil {
			return []byte{}, err
		}

		if int(lens) > v.Len() {
			zeroSlice := make([]byte, int(lens)-v.Len())
			data = append(data, zeroSlice...)
		}
		data = append(data, v.Bytes()...)
	case reflect.Struct:
		fieldCount := v.NumField()

		for i := 0; i < fieldCount; i++ {
			fmt.Println(v.Field(i).Type().String())
			d, err := refMarshal(v.Field(i), v.Type().Field(i))
			if err != nil {
				return []byte{}, err
			}

			data = append(data, d...)
		}
	}
	return data, nil
}

func Bytes2Word(data []byte) uint16 {
	if len(data) < 2 {
		return 0
	}
	return (uint16(data[0]) << 8) + uint16(data[1])
}

func Word2Bytes(data uint16) []byte {
	buff := make([]byte, 2)
	buff[0] = byte(data >> 8)
	buff[1] = byte(data)
	return buff
}

func Bytes2DWord(data []byte) uint32 {
	if len(data) < 4 {
		return 0
	}
	return (uint32(data[0]) << 24) + (uint32(data[1]) << 16) + (uint32(data[2]) << 8) + uint32(data[3])
}

func Dword2Bytes(data uint32) []byte {
	buff := make([]byte, 4)
	buff[0] = byte(data >> 24)
	buff[1] = byte(data >> 16)
	buff[2] = byte(data >> 8)
	buff[3] = byte(data)
	return buff
}
```



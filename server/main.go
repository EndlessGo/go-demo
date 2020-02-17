package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
)

type Initializer interface {
	Initialize()
}

func InitializeAll(initializers ...Initializer) {
	for _, initializer := range initializers {
		initializer.Initialize()
	}
}

type NamePwd struct {
	name string
	pwd  string
}
type DBManager struct {
	accountDB map[string]string //存储用户名密码
	//setch     chan map[string]string //DB的channel
	//getch     chan bool
	getNameCh   chan NamePwd
	getNameReCh chan bool
	setPwdCh    chan NamePwd
}

func (db *DBManager) Initialize() {
	db.accountDB = make(map[string]string)
	//db.dbch = make(chan map[string]string)
	db.getNameCh = make(chan NamePwd)
	db.getNameReCh = make(chan bool)
	db.setPwdCh = make(chan NamePwd)
	go db.ChannelOp()
	fmt.Print("DBManager Initialize\n")
	fmt.Println(db.accountDB)
}
func (db *DBManager) DBChekNamePwd(name string, pwd string) bool {
	db.getNameCh <- NamePwd{name, pwd}
	select {
	case ok := <-db.getNameReCh:
		fmt.Printf("DBChekNamePwd ok = %t\n", ok)
		return ok
	}
}
func (db *DBManager) DBSetName(name string, pwd string) {
	db.setPwdCh <- NamePwd{name, pwd}
	//TODO: need to return false, temporarily consider “	db.accountDB[name] = pwd” always success
	fmt.Print("DBSetName Success!\n")
}

func (db *DBManager) ChannelOp() {
	for {
		select {
		case result := <-db.getNameCh:
			{
				pwd, ok := db.accountDB[result.name]
				if ok {
					db.getNameReCh <- pwd == result.pwd
				} else {
					db.accountDB[result.name] = result.pwd
					db.getNameReCh <- true
				}

			}
		case val := <-db.setPwdCh:
			{
				//TODO: need to return false, cauze two terminal may DBChekName simultaneously,
				//so check if “db.accountDB[name] = pwd” already exist, if so return false caz register already
				db.accountDB[val.name] = val.pwd
				fmt.Println(db.accountDB)
			}
		}
	}
}

type OnlineInfo struct {
	//ternimalCount int
	ch   chan string
	conn net.Conn
}

type UserTerminal []OnlineInfo
type NameInfo struct {
	name string
	info OnlineInfo
}
type OnlineCache struct {
	nameChanMap        map[string]UserTerminal //用户名-通道-连接
	setNameInfoCh      chan NameInfo
	getLenCh           chan string
	getLenReCh         chan int
	popOnlineInfoCh    chan string
	deleteOnlineInfoCh chan string
}

func (olc *OnlineCache) Initialize() {
	olc.nameChanMap = make(map[string]UserTerminal)
	olc.setNameInfoCh = make(chan NameInfo)
	olc.getLenCh = make(chan string)
	olc.getLenReCh = make(chan int)
	olc.popOnlineInfoCh = make(chan string)
	olc.deleteOnlineInfoCh = make(chan string)
	go olc.ChannelOp()
	fmt.Print("OnlineCache Initialize\n")
	fmt.Println(olc.nameChanMap)
}
func (olc *OnlineCache) ChannelOp() {
	for {
		select {
		case val := <-olc.setNameInfoCh:
			{
				olc.OLCAppend(val.name, val.info)
			}
		case name := <-olc.getLenCh:
			{
				olc.getLenReCh <- len(olc.nameChanMap[name])
			}
		case name := <-olc.popOnlineInfoCh:
			{
				//出队操作
				olc.nameChanMap[name] = olc.nameChanMap[name][1:]
			}
		case name := <-olc.deleteOnlineInfoCh:
			{
				//delete only if the last conn
				fmt.Printf("OnlineCache delete last %s\n", name)
				delete(olc.nameChanMap, name)
			}
		}
	}
}

func (olc *OnlineCache) OLCAppend(name string, oinfo OnlineInfo) {
	olc.nameChanMap[name] = append(olc.nameChanMap[name], oinfo)
	count := len(olc.nameChanMap[name])
	if count > MaxTerminalCount {
		//close every conn
		for i := 0; i < count-MaxTerminalCount; i++ {
			//kickout previous player
			olc.nameChanMap[name][i].conn.Close()
		}
		//olc.nameChanMap[name] = olc.nameChanMap[name][len-MaxTerminalCount:len]
	}
}

func (olc *OnlineCache) OLCGetLen(name string) int {
	olc.getLenCh <- name
	select {
	case val := <-olc.getLenReCh:
		{
			return val
		}
	}
}

func (olc *OnlineCache) OLCDelete(name string) {
	olc.deleteOnlineInfoCh <- name
}

func (olc *OnlineCache) OLCPop(name string) {
	olc.popOnlineInfoCh <- name
}

type ConnHelper struct {
	db  DBManager
	olc OnlineCache
}

func (chp *ConnHelper) Initialize() {
	fmt.Print("ConnHelper Initialize\n")
	//chp.db = DBManager{}
	//chp.olc = OnlineCache{}
	InitializeAll(&chp.db, &chp.olc)
}
func (chp *ConnHelper) DBVerifyAccount(conn net.Conn) (name string, err error) {
	//先验证用户名密码，如果用户名首次出现则认为注册，否则验证密码正确性直到密码正确
	_, err = fmt.Fprintf(conn, "%s\r\n", "Hello, this is WonderServer. Please Input Your Name and Password!")
	input := bufio.NewScanner(conn)
	ok := false
	for !ok {
		input.Scan()
		name = input.Text()
		input.Scan()
		pwd := input.Text()

		ok = chp.DBChekNamePwd(name, pwd)
		if !ok {
			_, err = fmt.Fprintf(conn, "%s\r\n", name+": Name or Password Wrong!")
		} else {
			_, err = fmt.Fprintf(conn, "%s\r\n", name+": DBVerifyAccount Success!")
			_, err = fmt.Fprintf(conn, "%s\r\n", name+": Login Success!")
		}
	}
	return name, err
}
func (chp *ConnHelper) DBChekNamePwd(name string, pwd string) bool {
	return chp.db.DBChekNamePwd(name, pwd)
}
func (chp *ConnHelper) DBSetName(name string, pwd string) {
	//TODO: DBSetName Can be rename to change db name and password
	chp.db.DBSetName(name, pwd)
}
func (chp *ConnHelper) OnlineCacheAccountLogin(name string, conn net.Conn) OnlineInfo {
	oinfo := OnlineInfo{make(chan string), conn}
	//chp.OLCAppendAndPop(name, oinfo)
	chp.OLCAppend(name, oinfo)
	fmt.Fprintf(conn, "%s\r\n", name+": OnlineCacheAccountLogin Success!")
	return oinfo
}

func (chp *ConnHelper) OLCAppend(name string, oinfo OnlineInfo) {
	//chp.olc.OLCAppendAndPop(name, oinfo)
	chp.olc.setNameInfoCh <- NameInfo{name, oinfo}
	//TODO: need to wait success!
}

func (chp *ConnHelper) OLCGetLen(name string) int {
	return chp.olc.OLCGetLen(name)
}

func (chp *ConnHelper) OLCDelete(name string) {
	chp.olc.OLCDelete(name)
}

func (chp *ConnHelper) OLCPop(name string) {
	chp.olc.OLCPop(name)
}

var ( //like C++ Singleton
	cher             = ConnHelper{}
	MaxTerminalCount = 1
)

func main() {
	caster := BroadCaster{}
	cher.Initialize()
	caster.Initialize()
	listener, err := net.Listen("tcp", ":8000")
	if err != nil {
		log.Fatal(err)
	}
	go broadcaster(caster)
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		go handleConn(conn, caster)
	}
}

type client chan<- string //广播器
type BroadCaster struct {
	entering chan client
	leaving  chan client
	messages chan string     //所有接受的客户消息
	clients  map[client]bool //所有已连接的客户消息通道组成的map
}

func (bro *BroadCaster) Initialize() {
	bro.entering = make(chan client)
	bro.leaving = make(chan client)
	bro.messages = make(chan string)
	bro.clients = make(map[client]bool) //所有已连接的客户消息通道组成的map
	fmt.Print("BroadCaster Initialize\n")
}

func broadcaster(caster BroadCaster) {
	//clients := make(map[client]bool)//所有已连接的客户消息通道组成的map
	for {
		select {
		case msg := <-caster.messages:
			for cli := range caster.clients {
				cli <- msg
			}
		case cli := <-caster.entering:
			caster.clients[cli] = true
		case cli := <-caster.leaving:
			if _, ok := caster.clients[cli]; ok {
				fmt.Printf("leaving once!\n")
				delete(caster.clients, cli)
				close(cli)
			} else {
				fmt.Printf("leaving twice!\n")
			}
		}
	}
}

func handleConn(conn net.Conn, caster BroadCaster) {
	//First verify account correctly!
	//DBVerifyAccount must contain and solve error
	name, err := cher.DBVerifyAccount(conn)
	if err != nil {
		log.Fatal(err)
	}
	//direct add user to olc, if greater than default, pop front
	oinfo := cher.OnlineCacheAccountLogin(name, conn)

	go clientWriter(conn, oinfo.ch)

	//ip := conn.RemoteAddr().String();
	oinfo.ch <- "Have Fun! " + name
	caster.messages <- name + " has arrived"
	caster.entering <- oinfo.ch

	input := bufio.NewScanner(conn)
	for input.Scan() {
		caster.messages <- name + ": " + input.Text()
	}

	caster.leaving <- oinfo.ch
	caster.messages <- name + " has left"
	conn.Close()

	cher.OLCPop(name)

	fmt.Printf(name+" conn Close! OLCGetLen = %d\n", cher.OLCGetLen(name))
	if cher.OLCGetLen(name) == 0 {
		cher.OLCDelete(name)
	}
}

func clientWriter(conn net.Conn, ch chan string) {
	for msg := range ch {
		fmt.Fprintf(conn, "%s\r\n", msg)
	}
}

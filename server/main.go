package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"sync"
)

type Initializer interface {
	Initialize()
}

func InitializeAll(initializers...Initializer) {
	for _, initializer := range initializers{
		initializer.Initialize()
	}
}

type GetNameRe struct {
	pwd string
	ok bool
}
type NamePwd struct {
	name string
	pwd string
}
type DBManager struct {
	accountDB map[string]string //存储用户名密码
	//setch     chan map[string]string //DB的channel
	//getch     chan bool
	getNameCh   chan string
	getNameReCh chan GetNameRe
	setPwdCh    chan NamePwd
}
func (db *DBManager) Initialize(){
	db.accountDB = make(map[string]string)
	//db.dbch = make(chan map[string]string)
	db.getNameCh = make(chan string)
	db.getNameReCh = make(chan GetNameRe)
	db.setPwdCh = make(chan NamePwd)
	go db.ChannelOp()
	fmt.Print("DBManager Initialize\n")
	fmt.Println(db.accountDB)
}
func (db *DBManager) DBChekName(name string) (string, bool) {
	db.getNameCh<-name
	select {
	case result := <- db.getNameReCh:
		return result.pwd, result.ok
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
		case name := <-db.getNameCh:{
			pwd, ok := db.accountDB[name]
			db.getNameReCh <- GetNameRe{pwd, ok}
		}
		case val:= <-db.setPwdCh:{
			//TODO: need to return false, cauze two terminal may DBChekName simultaneously,
			//so check if “db.accountDB[name] = pwd” already exist, if so return false caz register already
			db.accountDB[val.name] = val.pwd
		}
		}
	}
}

type OnlineInfo struct {
	//ternimalCount int
	ch chan string
	conn net.Conn
}

type UserTerminal []OnlineInfo
type NameInfo struct {
	name string
	info OnlineInfo
}
type OnlineCache struct {
	nameChanMap   map[string]UserTerminal //用户名-通道-连接
	mutex         *sync.RWMutex
	setNameInfoCh chan NameInfo
	getLenCh      chan string
	getLenReCh	  chan int
}
func (olc *OnlineCache) Initialize(){
	olc.nameChanMap = make(map[string]UserTerminal)
	olc.setNameInfoCh = make(chan NameInfo)
	olc.getLenCh = make(chan string)
	olc.getLenReCh = make(chan int)
	go olc.ChannelOp()
	olc.mutex = &sync.RWMutex{}
	fmt.Print("OnlineCache Initialize\n")
	fmt.Println(olc.nameChanMap)
}
func (olc *OnlineCache) ChannelOp() {
	for {
		select {
		case  val := <-olc.setNameInfoCh:{
			olc.OLCAppendAndPop(val.name, val.info)
		}
		case name := <-olc.getLenCh:{
			olc.getLenReCh<- len(olc.nameChanMap[name])
		}
		}
	}
}
func (olc *OnlineCache) OLCGetByName(name string) (oi UserTerminal,ok bool) {
	olc.mutex.Lock()
	defer olc.mutex.Unlock()
	oi, ok = olc.nameChanMap[name]
	return oi, ok
}

func (olc *OnlineCache) OLCAppendAndPop(name string,oinfo OnlineInfo) {
	olc.nameChanMap[name] = append(olc.nameChanMap[name], oinfo)
	len := len(olc.nameChanMap[name])
	if len > MaxTerminalCount{
		//close every conn
		for i:=0; i < len-MaxTerminalCount;i++{
			//kickout previous player
			olc.nameChanMap[name][i].conn.Close()
		}
		olc.nameChanMap[name] = olc.nameChanMap[name][len-MaxTerminalCount:len]
	}
}

func (olc *OnlineCache) OLCGetLen(name string) int {
	olc.getLenCh <- name
	select {
	case val := <- olc.getLenReCh:{
		return val
	}
	}
}

func (olc *OnlineCache) OLCDelete(name string) {
	olc.mutex.Lock()
	defer olc.mutex.Unlock()
	delete(olc.nameChanMap,name)//delete only if the last conn
}

type ConnHelper struct {
	db DBManager
	olc OnlineCache
}
func (chp *ConnHelper) Initialize(){
	fmt.Print("ConnHelper Initialize\n")
	//chp.db = DBManager{}
	//chp.olc = OnlineCache{}
	InitializeAll(&chp.db, &chp.olc)
}
func (chp *ConnHelper) DBVerifyAccount(conn net.Conn) string{
	//先验证用户名密码，如果用户名首次出现则认为注册，否则验证密码正确性直到密码正确
	fmt.Fprintf(conn, "%s\r\n", "Hello, this is WonderServer. Please Input Your Name!")
	input := bufio.NewScanner(conn)
	input.Scan()
	name := input.Text()
	pwd, ok := chp.DBChekName(name)
	if !ok {
		fmt.Fprintf(conn, "%s\r\n", name+": Register Success!")
		fmt.Fprintf(conn, "%s\r\n", name+": Please Register Your Password!")
		input.Scan()
		pwd = input.Text()
		//db.accountDB[name] = pwd
		chp.DBSetName(name,pwd)
	} else{
		fmt.Fprintf(conn, "%s\r\n", name+": Old Player Welcome!")
		fmt.Fprintf(conn, "%s\r\n", name+": Please Input Your Password!")
		input.Scan()
		for input.Text() != pwd{
			fmt.Fprintf(conn, "%s\r\n", "Password Wrong! Please Input Again!")
			input.Scan()
		}
	}
	fmt.Fprintf(conn, "%s\r\n", name+": DBVerifyAccount Success!")
	return name
}
func (chp *ConnHelper) DBChekName(name string) (string, bool) {
	return chp.db.DBChekName(name)
}
func (chp *ConnHelper) DBSetName(name string, pwd string) {
	chp.db.DBSetName(name,pwd)
}
func (chp *ConnHelper)OnlineCacheAccountLogin(name string, conn net.Conn) OnlineInfo {
	oinfo := OnlineInfo{make(chan string),conn}
	chp.OLCAppendAndPop(name, oinfo)
	fmt.Fprintf(conn, "%s\r\n", name+": OnlineCacheAccountLogin Success!")
	return oinfo
}
func (chp *ConnHelper) OLCGetByName(name string) (UserTerminal, bool) {
	return chp.olc.OLCGetByName(name)
}
func (chp *ConnHelper) OLCAppendAndPop(name string, oinfo OnlineInfo) {
	//chp.olc.OLCAppendAndPop(name, oinfo)
	chp.olc.setNameInfoCh <- NameInfo{name, oinfo}
	//TODO: need to wait success!
}

func (chp *ConnHelper) OLCGetLen(name string) int{
	return chp.olc.OLCGetLen(name)
}

func (chp *ConnHelper) OLCDelete(name string) {
	chp.olc.OLCDelete(name)
}

var (//like C++ Singleton
	cher = ConnHelper{}
	MaxTerminalCount = 1
)

func main()  {
	caster := BroadCaster{}
	cher.Initialize()
	caster.Initialize()
	listener, err := net.Listen("tcp", ":8000")
	if err != nil{
		log.Fatal(err)
	}
	go broadcaster(caster)
	for{
		conn, err := listener.Accept()
		if err != nil{
			log.Print(err)
			continue
		}
		go handleConn(conn, caster)
	}
}

type client chan<-string//广播器
type BroadCaster struct{
	entering chan client
	leaving chan client
	messages chan string//所有接受的客户消息
	clients map[client]bool//所有已连接的客户消息通道组成的map
}
func (bro *BroadCaster) Initialize() {
	bro.entering = make(chan client)
	bro.leaving = make(chan client)
	bro.messages = make(chan string)
	bro.clients = make(map[client]bool)//所有已连接的客户消息通道组成的map
	fmt.Print("BroadCaster Initialize\n")
}

func broadcaster(caster BroadCaster) {
	//clients := make(map[client]bool)//所有已连接的客户消息通道组成的map
	for {
		select {
		case msg := <-caster.messages:
			for cli := range caster.clients{
				cli <- msg
			}
		case cli := <-caster.entering:
			caster.clients[cli] = true
		case cli := <-caster.leaving:
			if _,ok:=caster.clients[cli]; ok{
				fmt.Printf("leaving once!\n")
				delete(caster.clients,cli)
				close(cli)
			}else{
				fmt.Printf("leaving twice!\n")
			}
		}
	}
}

func handleConn(conn net.Conn, caster BroadCaster) {
	//First verify account correctly!
	//TODO: DBVerifyAccount must contain and solve error
	name := cher.DBVerifyAccount(conn)
	//direct add user to olc, if greater than default, pop front
	oinfo := cher.OnlineCacheAccountLogin(name, conn)

	go clientWriter(conn, oinfo.ch)

	//ip := conn.RemoteAddr().String();
	oinfo.ch <- "Have Fun! " + name
	caster.messages <- name + " has arrived"
	caster.entering <- oinfo.ch

	input := bufio.NewScanner(conn)
	for input.Scan(){
		caster.messages <- name + ": " + input.Text()
	}

	caster.leaving <- oinfo.ch
	caster.messages <- name + " has left"
	conn.Close()

	fmt.Printf(name+" conn Close! OLCGetLen = %d\n", cher.OLCGetLen(name))
	if cher.OLCGetLen(name) == 0 {
		cher.OLCDelete(name)
	}
}

func clientWriter(conn net.Conn, ch chan string) {
	for msg := range ch{
		fmt.Fprintf(conn, "%s\r\n", msg)
	}
}
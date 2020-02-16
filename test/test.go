/*
package main


type OnlineInfo struct {
	//isOnline bool
	//ch chan string
	value int
}

var nameChanMap = make(map[string]OnlineInfo)


func main()  {
	val1 := OnlineInfo{1}
	nameChanMap["w1"] = val1;
	println(nameChanMap["w1"].value)//1
	val1.value = 2
	println(nameChanMap["w1"].value)//1 caz no change
	temp := MapChange(nameChanMap["w1"],3)
	nameChanMap["w1"] = temp
	println(nameChanMap["w1"].value)//3
}

func MapChange(info OnlineInfo, i int) OnlineInfo{
	info.value = i
	return info
}
*/

package main
import "fmt"
type Vertex struct {
	X int
	Y int
}
func main() {
	p := Vertex{1, 2}
	q := &p
	q.X = 1e9
	fmt.Println(p)
}
package msgBus

import(
	mbus "github.com/asaskevich/EventBus"
	"HNB/logging"
)


var mBus mbus.Bus
var myLog logging.LogModule
const (
	MSGBUS string = "msgBus"
)

func InitMsgBus(){
	mBus = mbus.New()
	myLog = logging.GetLogIns()
}

//要求订阅method的方法必须是两个参数
func Subscribe(key string, method interface{}){

	myLog.Info(MSGBUS, "subscribe topic: " + key)
	err := mBus.Subscribe(key, method)
	if err != nil{
		panic("msg bus subscribe err: " + err.Error())
	}
}

func Publish(key string, args interface{}, result interface{}){
	mBus.Publish(key, args, result)
}


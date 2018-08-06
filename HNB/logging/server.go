package logging


var globalLogModule LogModule

//log module interface
type LogModule interface{
	Info(key, msg string)
	Debug(key, msg string)
	Warning(key, msg string)
	Error(key, msg string)
}

//other module get log instance
func GetLogIns() LogModule {
	return globalLogModule
}


func InitLogModule(){
	li := &logrusIns{}
	li.Init()
	globalLogModule = li
}

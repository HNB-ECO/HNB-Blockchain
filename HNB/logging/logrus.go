package logging

import (
	"time"
	"github.com/lestrrat/go-file-rotatelogs"
	"github.com/rifflock/lfshook"
	"os"
	"bufio"
	log "github.com/sirupsen/logrus"
	"path"
	"HNB/config"
	"HNB/util"
)


type logrusIns struct{
	*log.Logger
}


func (li *logrusIns) Init(){
	//根据config文件配置读取日志路径、日志文件名称、以及切割日期间隔、日志级别
	if !util.PathExists(config.Config.Log.Path){
		os.Mkdir(config.Config.Log.Path, os.ModePerm)
	}
	baseLogPaht := path.Join(config.Config.Log.Path, "log")
	writer, err := rotatelogs.New(
		baseLogPaht+".%Y%m%d%H%M%S",
		rotatelogs.WithLinkName(baseLogPaht), // 生成软链，指向最新日志文件
		//rotatelogs.WithMaxAge(maxAge), // 文件最大保存时间
		rotatelogs.WithRotationTime(24 * time.Hour), // 日志切割时间间隔
	)
	if err != nil {
		panic("config logrus err: " + err.Error())
	}

	logNew := log.New()
	lfHook := lfshook.NewHook(lfshook.WriterMap{
		log.DebugLevel: writer,
		log.InfoLevel:  writer,
		log.WarnLevel:  writer,
		log.ErrorLevel: writer,
		log.FatalLevel: writer,
		log.PanicLevel: writer,
	}, &log.JSONFormatter{})

	logNew.SetLevel(LevelConv(config.Config.Log.Level))

	src, err := os.OpenFile(os.DevNull, os.O_APPEND|os.O_WRONLY|os.O_CREATE, os.ModeAppend)
	if err!= nil{
		panic("config openfile err: " + err.Error())
	}
	writer1 := bufio.NewWriter(src)

	logNew.SetOutput(writer1)

	logNew.AddHook(lfHook)

	li.Logger = logNew

}

func LevelConv(level string) log.Level{
	switch level {
	case "debug":
		return log.DebugLevel
	case "info":
		return log.InfoLevel
	case "warning":
		return log.WarnLevel
	case "error":
		return log.ErrorLevel
	}
	panic("log level invalid,level: " + level)
}

func (li *logrusIns) Debug(key, msg string){
	li.WithFields(log.Fields{"type": key}).Debug(msg)
}

func (li *logrusIns) Info(key, msg string){
	li.WithFields(log.Fields{"type": key}).Info(msg)
}

func (li *logrusIns) Warning(key, msg string){
	li.WithFields(log.Fields{"type": key}).Warning(msg)
}

func (li *logrusIns) Error(key, msg string){
	li.WithFields(log.Fields{"type": key}).Error(msg)
}

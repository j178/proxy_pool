package util

import (
	"os"

	"github.com/apex/log"
	"github.com/apex/log/handlers/cli"
	"github.com/fatih/structs"
)

var logger *log.Logger

func GetLogger(module string) *log.Entry {
    if logger == nil {
        var level log.Level
        if ServerConf.Debug {
            level = log.DebugLevel
        } else {
            level = log.InfoLevel
        }
        logger = &log.Logger{
            Level: level,
            Handler: cli.New(os.Stdout),
        }
        logger.WithField("config", structs.Map(ServerConf)).Info("loaded config")
    }

    return logger.WithField("module", module)
}

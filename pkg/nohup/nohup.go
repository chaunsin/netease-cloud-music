package nohup

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Close interface {
	Close(ctx context.Context) error
}

type CloseHook func(ctx context.Context) error

func (f CloseHook) Close(ctx context.Context) error {
	return f(ctx)
}

func Run(close ...Close) {
	path, err := os.Executable()
	if err != nil {
		panic(err)
	}
	log.Printf("Program run directory: %s", path)

	fd, err := os.Create("./pid")
	if err != nil {
		panic(err)
	}
	defer fd.Close()
	if _, err := fd.WriteString(fmt.Sprintf("%d", os.Getpid())); err != nil {
		panic(err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT)
	for s := range c {
		log.Printf("[nohup] get a signal %s\n", s.String())
		switch s {
		case syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGINT:
			for _, v := range close {
				v.Close(context.Background())
			}
			time.Sleep(time.Second * 1)
			log.Printf("[nohup] EXIT...")
			return
		case syscall.SIGHUP:
		default:
			return
		}
	}
}

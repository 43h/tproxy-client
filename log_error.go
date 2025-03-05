//go:build !debug

package main

import (
	"log"
)

func LOGD(v ...interface{}) {
}

func LOGI(v ...interface{}) {
}

func LOGE(v ...interface{}) {
	if logLevel <= ERROR {
		log.Println("[ERROR] ", v)
	}
}

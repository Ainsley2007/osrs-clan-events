package discord

import "log"

func goSafe(label string, fn func()) {
	go func() {
		defer func() {
			if v := recover(); v != nil {
				log.Printf("panic in %s: %v", label, v)
			}
		}()
		fn()
	}()
}

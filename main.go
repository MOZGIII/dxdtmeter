package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func countHandleFunc(countch chan<- struct{}) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, "OK"); err != nil {
			log.Print(err)
		}
		countch <- struct{}{}
	}
}

func pokeHandleFunc(pokech chan<- chan<- uint64) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		valch := make(chan uint64)
		pokech <- valch
		val := <-valch
		if _, err := fmt.Fprint(w, val); err != nil {
			log.Print(err)
		}
	}
}

func countall(ch <-chan struct{}, pokech <-chan chan<- uint64) uint64 {
	var count uint64
	count = 0
loop:
	for {
		select {
		case _, ok := <-ch:
			if ok {
				count++
				log.Println(count)
			} else {
				break loop
			}
		case valch := <-pokech:
			valch <- count
		}
	}
	return count
}

func httpserver(addr string, countch chan struct{}, pokech chan<- chan<- uint64) <-chan error {
	errch := make(chan error)
	go func() {
		http.HandleFunc("/", pokeHandleFunc(pokech))
		http.HandleFunc("/inc", countHandleFunc(countch))
		errch <- http.ListenAndServe(addr, nil)
		close(errch)
	}()
	return errch
}

func sigch(signals ...os.Signal) <-chan os.Signal {
	outch := make(chan os.Signal, 1)
	signal.Notify(outch, signals...)
	return outch
}

func dumpcount(pokech chan<- chan<- uint64) {
	valch := make(chan uint64)
	pokech <- valch
	val := <-valch
	rendercount(val)
}

func rendercount(count uint64) {
	fmt.Printf("Current counter value: %d\n", count)
}

func main() {
	countch := make(chan struct{}, 32)
	pokech := make(chan chan<- uint64)
	go countall(countch, pokech)

	httperrch := httpserver(":6553", countch, pokech)
	termch := sigch(syscall.SIGTERM, syscall.SIGINT)
	usr1ch := sigch(syscall.SIGUSR1)

mainloop:
	for {
		select {
		case sig := <-usr1ch:
			log.Printf("Got %v\n", sig)
			dumpcount(pokech)
		case sig := <-termch:
			log.Printf("Got %v\n", sig)
			break mainloop
		case err := <-httperrch:
			if err != nil {
				log.Println(err)
			}
			break mainloop
		}
	}

	dumpcount(pokech)
	close(countch)
}

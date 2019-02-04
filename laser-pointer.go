package main

import "flag"
import "fmt"
import "github.com/pugmajere/pantilthat"
import "github.com/stianeikeland/go-rpio"
import "log"
import "math/rand"
import "net/http"
import "strings"
import "sync"
import "time"

const (
	gpio_laser1 = 18
)

var hat *pantilthat.PanTiltHat
var hatLock sync.Mutex
var durationFlag *time.Duration
var laserPin rpio.Pin

func simplePattern(hat *pantilthat.PanTiltHat) {

	var pan_angle, tilt_angle, i int16

	for i = 0; i < 90; i++ {
		pan_angle = i - 90
		tilt_angle = i/2 + 45 // Range from 45-90

		hat.Pan(pan_angle)
		hat.Tilt(tilt_angle)
		time.Sleep(time.Second / 20)
	}

	for i = 0; i < 90; i++ {
		pan_angle = 0 - i
		tilt_angle = 90 - i/2 // Range from 90-45

		hat.Pan(pan_angle)
		hat.Tilt(tilt_angle)
		time.Sleep(time.Second / 20)
	}
}

func randBool() bool {
	return rand.Float32() < 0.5
}

func adjustAroundCenter(center, max, min int16) int16 {
	var adjust, result int16

	if center == max {
		adjust = -2
	} else if center == min {
		adjust = 2
	} else {
		if randBool() {
			adjust = -2
		} else {
			adjust = 2
		}
	}

	result = center + adjust
	return result
}

func linePattern(hat *pantilthat.PanTiltHat) {
	var pan, i int16

	pan = -30
	hat.Pan(pan)
	for i = 45; i < 90; i++ {
		hat.Tilt(i)
		pan = adjustAroundCenter(pan, -40, -20)
		hat.Pan(pan)
		time.Sleep(time.Second / 15)
		log.Printf("tilt = %d\n", i)
	}

	for i = 0; i < 45; i++ {
		hat.Tilt(90 - i)
		pan = adjustAroundCenter(pan, -40, -20)
		hat.Pan(pan)
		time.Sleep(time.Second / 15)
		log.Printf("tilt = %d\n", i)
	}
}

func triggerLaser(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	fmt.Println(r.Form) // print form information in server side
	fmt.Println("path", r.URL.Path)
	fmt.Println("scheme", r.URL.Scheme)
	fmt.Println(r.Form["url_long"])
	for k, v := range r.Form {
		fmt.Println("key:", k)
		fmt.Println("val:", strings.Join(v, ""))
	}

	if strings.Join(r.Form["laser"], "") != "" {
		fmt.Fprintf(w, "Laser Active!")
		go func() {
			hatLock.Lock()
			defer hatLock.Unlock()
			laserPin.High()
			defer laserPin.Low()

			now := time.Now()
			for time.Since(now) < *durationFlag {
				time.Sleep(time.Second)
			}
			log.Println("timeout")
		}()

	} else {
		fmt.Fprintf(w, "Oh Laser. (set laser=1 for laser.")
	}
}

func triggerCats(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	if strings.Join(r.Form["cats"], "") == "1" {
		go func() {
			hatLock.Lock()
			defer hatLock.Unlock()
			laserPin.High()
			defer laserPin.Low()
			defer hat.ServoEnable(1, false)
			defer hat.ServoEnable(2, false)

			now := time.Now()
			for time.Since(now) < *durationFlag {
				//simplePattern(hat)
				linePattern(hat)
			}
			log.Println("timeout")
		}()
	} else {
		fmt.Fprintf(w, "To entertain cats, retry with cats=1")
	}
}

func main() {
	durationFlag = flag.Duration("duration", 20*(1000*1000*1000), "Duration that the laser should move.")

	flag.Parse()

	rand.Seed(time.Now().UnixNano())
	fmt.Println(randBool())

	err := rpio.Open()
	if err != nil {
		log.Printf("rpio failed: %s\n", err)
	} else {
		log.Println("rpio ok")
	}
	defer rpio.Close()

	pin := rpio.Pin(gpio_laser1)
	pin.Output()
	defer pin.Low()
	laserPin = pin

	hat, err = pantilthat.MakePanTiltHat(&pantilthat.PanTiltHatParams{})
	if err != nil {
		log.Printf("error init: %s\n", err)
		return
	}
	defer hat.Close()

	http.HandleFunc("/laser", triggerLaser)
	http.HandleFunc("/cats", triggerCats)

	fmt.Println("Start serving.")
	err = http.ListenAndServeTLS(":8443", "cert.pem", "key.pem", nil) // set listen port
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}

}

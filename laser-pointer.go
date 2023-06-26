package main

import "context"
import "flag"
import "fmt"
import "github.com/pugmajere/pantilthat"
import "github.com/stianeikeland/go-rpio"
import "html/template"
import "log"
import "math"
import "math/rand"
import "net/http"
import "strings"
import "sync"
import "time"

const (
	gpio_laser1 = 18   // pin
	m           = -0.41
	b           = 62.67

	minA   = 26
	maxA   = 81
	minX   = 0.5
	maxX   = 15.0
	deltaX = 0.05

	startPan = -30
	minPan   = -20
	maxPan   = -40
)

var hat *pantilthat.PanTiltHat
var hatLock sync.Mutex
var activeLock sync.Mutex
var active bool
var durationFlag *time.Duration
var laserPin rpio.Pin
var tmpl *template.Template

type PageData struct {
	LaserStatus string
}

func SetActive(value bool) {
	activeLock.Lock()
	defer activeLock.Unlock()
	active = value
}

func Deg(r float64) float64 {
	return r / (math.Pi / 180)
}

func convertRealDegreesIntoTilt(theta float64) float64 {
	return m*theta + b
}

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

	pan = startPan
	hat.Pan(pan)
	for i = 45; i < 90; i++ {
		hat.Tilt(i)
		pan = adjustAroundCenter(pan, minPan, maxPan)
		hat.Pan(pan)
		time.Sleep(time.Second / 15)
		log.Printf("tilt = %d\n", i)
	}

	for i = 0; i < 45; i++ {
		hat.Tilt(90 - i)
		pan = adjustAroundCenter(pan, minPan, maxPan)
		hat.Pan(pan)
		time.Sleep(time.Second / 15)
		log.Printf("tilt = %d\n", i)
	}
}

func adjustTargetToX(x float64, pan *int16) {
	theta := Deg(math.Atan(x / 3))
	thetaPrime := convertRealDegreesIntoTilt(theta)
	hat.Tilt(int16(thetaPrime))
	*pan = adjustAroundCenter(*pan, minPan, maxPan)
	hat.Pan(*pan)
	time.Sleep(time.Second / 15)
	log.Printf("tilt = %f, real = %f, x = %f\n", thetaPrime, theta, x)

}

func smoothLinePattern(hat *pantilthat.PanTiltHat) {
	var pan int16
	var x float64
	pan = startPan
	hat.Pan(pan)

	for x = minX; x < maxX; x += deltaX {
		adjustTargetToX(x, &pan)
	}

	for x = maxX; x > minX; x -= deltaX {
		adjustTargetToX(x, &pan)
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

func runCatLaser() {
	hatLock.Lock()
	defer hatLock.Unlock()
	laserPin.High()
	defer laserPin.Low()
	defer hat.ServoEnable(1, false)
	defer hat.ServoEnable(2, false)
	defer SetActive(false)

	now := time.Now()
	for time.Since(now) < *durationFlag {
		//simplePattern(hat)
		smoothLinePattern(hat)
	}
	log.Println("timeout")

}

func triggerCats(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	if strings.Join(r.Form["cats"], "") == "1" {
		if !active {
			SetActive(true)
			go runCatLaser()
		}
	}
	activeLock.Lock()
	defer activeLock.Unlock()
	data := PageData{}
	if active {
		data.LaserStatus = "Active"
	} else {
		data.LaserStatus = "Sleeping"
	}
	tmpl.Execute(w, data)

}

func main() {
	durationFlag = flag.Duration("duration", 20*(1000*1000*1000), "Duration that the laser should move.")
	sslFlag := flag.Bool("use_ssl", false, "Enable SSL")

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

	// Initialize the laser control pin, and defer turning it off.
	pin := rpio.Pin(gpio_laser1)
	pin.Output()
	defer pin.Low()
	laserPin = pin

	active = false

	// Set up HTML template files.
	tmpl = template.Must(template.ParseFiles("tmpl/page.html"))

	// Set up PanTiltHat controls.
	hat, err = pantilthat.MakePanTiltHat(&pantilthat.PanTiltHatParams{})
	if err != nil {
		log.Printf("error init: %s\n", err)
		return
	}
	defer hat.Close()

	// Schedule a periodic automatic run, so the cats have fun every day:
	ctx := context.Context(context.Background())
	// Run at 11pm PST (10pm PDT)
	crontime := time.Hour * 7
	go Schedule(ctx, time.Hour*24, crontime, func(_ time.Time) { runCatLaser() })
	defer ctx.Done()

	// Set up HTTP(S) server:
	// (must be the last thing in main())
	http.HandleFunc("/laser", triggerLaser)
	http.HandleFunc("/cats", triggerCats)
	http.HandleFunc("/", triggerCats)

	fmt.Println("Start serving.")
	if *sslFlag {
		err = http.ListenAndServeTLS(":8443", "cert.pem", "key.pem", nil) // set listen port
	} else {
		err = http.ListenAndServe(":8000", nil) // set listen port
	}
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}

}

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httptrace"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	// "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	// "k8s.io/client-go/tools/clientcmd"
)

var (
	helpFlag      = flag.Bool("help", false, "")
	endpointsName = flag.String("endpointsname", "shelob", "Endpoints object name, usually the servicename this pod belongs to.")
	period        = flag.String("period-duration", "1s", "Standard golang duration definition, determins timing between endpoint tests.")
	httpPort      = flag.Int("port", 8080, "Port to serve the test (/) and metrics (/metrics) endpoints on")

	clientset       *kubernetes.Clientset
	requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "test_request_druation_ms",
		Help: "Histogram 1 to 10000 linear ms.",
		Buckets: []float64{
			1,
			2,
			3,
			4,
			5,
			6,
			7,
			8,
			9,
			10,
			11,
			12,
			13,
			14,
			15,
			16,
			17,
			18,
			19,
			20,
			25,
			30,
			35,
			40,
			45,
			50,
			55,
			60,
			65,
			70,
			75,
			80,
			85,
			90,
			95,
			100,
			110,
			120,
			130,
			140,
			150,
			160,
			170,
			180,
			190,
			200,
			250,
			300,
			350,
			400,
			450,
			500,
			600,
			700,
			800,
			900,
			1000,
			2000,
			3000,
			4000,
			5000,
			10000,
		},
	}, []string{"source", "destination"})
)

func main() {
	flag.Parse()

	if *helpFlag {
		flag.PrintDefaults()
		os.Exit(1)
	}

	// set up metrics
	prometheus.MustRegister(requestDuration)

	log.Printf("Watched endpoint: %s\n", *endpointsName)

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Error building kubeconfig: %s\n", err.Error())
	}

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error build client: %s\n", err)
	}

	go testLoop()

	// handlers
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})
	http.Handle("/metrics", prometheus.Handler())
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *httpPort), nil))

}

func testLoop() {
	periodDuration, err := time.ParseDuration(*period)
	if err != nil {
		log.Printf("Error: Value of --period-duration cannot be parsed, see https://golang.org/pkg/time/#ParseDuration, using default value of '1s'. Passed value: %s Error: %s\n", *period, err)
		periodDuration = time.Second
	}
	for {
		endpoint, err := clientset.CoreV1().Endpoints("utils").Get(*endpointsName, meta_v1.GetOptions{})
		if err != nil {
			log.Printf("Unable to fetch endpoint: %s\n", err)
			continue
		}
		for _, subset := range endpoint.Subsets {
			for _, address := range subset.Addresses {
				go measureLatency(address.IP, *httpPort)
			}
		}
		time.Sleep(periodDuration)
	}
}

func measureLatency(ip string, port int) {
	var connStart, firstByte time.Time

	trace := &httptrace.ClientTrace{
		ConnectStart: func(network string, addr string) {
			connStart = time.Now() // Measure time immediately and once
		},
		GotFirstResponseByte: func() {
			firstByte = time.Now()
		},
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s:%d/", ip, port), nil)
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	res, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		log.Printf("Error measuring latency to %s:%d %s\n", ip, port, err)
	}
	mytime := firstByte.Sub(connStart)
	res.Body.Close()
	if res.StatusCode != 200 {
		log.Printf("Unexpected status code %d while measuring latency of %s:%d\n", res.StatusCode, ip, port)
	}
	// log.Printf("%s->%s=%f %d %d\n", os.Getenv("PODIP"), ip, (mytime.Seconds() * 1e3), connStart.Unix(), firstByte.Unix())
	requestDuration.With(prometheus.Labels{"source": os.Getenv("PODIP"), "destination": ip}).Observe(mytime.Seconds() * 1e3)
}

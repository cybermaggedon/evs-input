package main

import (
	"bufio"
	"encoding/binary"
	"github.com/cybermaggedon/evs-golang-api"
	pb "github.com/cybermaggedon/evs-golang-api/protos"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/prometheus/client_golang/prometheus"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

type InputConfig struct {
	*evs.Config
	Port uint16
}

func NewInputConfig() *InputConfig {
	ic := &InputConfig{
		Config: evs.NewConfig("evs-input", "NOT-USED",
			[]string{"cyberprobe"}),
		Port: 6789,
	}

	if val, ok := os.LookupEnv("INPUT_PORT"); ok {
		port, _ := strconv.Atoi(val)
		ic.Port = uint16(port)
	}

	return ic

}

// Input analytic, just inputs out a kinda JSON equivalent of the event message.
type Input struct {
	*InputConfig
	*evs.EventProducer
	evs.Interruptible

	// false to cause event loops to stop
	running bool

	// queue between receiver and publisher
	ch chan []byte

	// Metrics
	event_latency prometheus.Summary
	received      prometheus.Counter
}

func NewInput(ic *InputConfig) *Input {

	i := &Input{InputConfig: ic, running: true}

	i.ch = make(chan []byte, 1000)

	var err error
	i.EventProducer, err = evs.NewEventProducer(i)
	if err != nil {
		log.Fatal(err)
	}

	i.RegisterStop(i)

	i.event_latency = prometheus.NewSummary(
		prometheus.SummaryOpts{
			Name: "input_event_latency",
			Help: "Latency from probe to input",
		})
	prometheus.MustRegister(i.event_latency)

	i.received = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "input_received",
			Help: "Numer of events received",
		})
	prometheus.MustRegister(i.received)

	return i
}

func (i *Input) Stop() {
	i.running = false
}

// Handler for one client TCP connection
func (i *Input) Stream(conn net.Conn) {

	// Stream format is a length field (4-byte network byte order plus
	// a payload in protobuf format.

	defer conn.Close()

	maxlen := uint64(128 * 1024 * 1024)

	var (
		lenb = make([]byte, 4)
		r    = bufio.NewReader(conn)
	)

	for {
		n, err := io.ReadFull(r, lenb)

		if err != nil {
			if err != io.EOF {
				log.Print(err)
			}
			break
		}

		if n != 4 {
			log.Print("Truncation?!")
			break
		}

		l := uint64(binary.BigEndian.Uint32(lenb))

		if l == 0 {
			log.Print("len == 0?!")
			break
		}

		if l > maxlen {
			log.Print("PDU too long")
			break
		}

		buf := make([]byte, l)
		n, err = io.ReadFull(r, buf)

		if err != nil {
			log.Print(err)
			if err != io.EOF {
				log.Print(err)
			}
			break
		}

		if uint64(n) != l {
			log.Print("Truncation?!")
			break
		}

		i.received.Inc()

		i.ch <- buf

	}

}

func (i *Input) Xfer() {

	for buf := range i.ch {

		ev := &pb.Event{}
		err := proto.Unmarshal(buf, ev)
		if err != nil {
			log.Print(err)
			continue
		}

		go i.recordLatency(time.Now(), ev)

		props := map[string]string{
			"device":  ev.Device,
			"network": ev.Network,
		}

		i.Output(ev, props)

	}

}

func (i *Input) recordLatency(ts time.Time, ev *pb.Event) {
	tm, _ := ptypes.Timestamp(ev.Time)
	latency := ts.Sub(tm)
	i.event_latency.Observe(float64(latency))
}

func (i *Input) Run() {

	lsnr, err := net.Listen("tcp", ":"+strconv.Itoa(int(i.Port)))

	if err != nil {
		log.Print(err)
		panic(err)
	}

	go i.Xfer()

	tcplsnr := lsnr.(*net.TCPListener)

	for i.running {

		tcplsnr.SetDeadline(time.Now().Add(time.Millisecond * 100))

		conn, _ := tcplsnr.Accept()

		if conn != nil {
			go i.Stream(conn)
		}

	}

}

func main() {

	ic := NewInputConfig()
	i := NewInput(ic)
	i.Run()
	log.Print("Shutdown.")

}

package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	"layeh.com/gumble/gumble"
	"layeh.com/gumble/gumbleutil"
	_ "layeh.com/gumble/opus"
	"net"
	"os"
	"strconv"
	"time"
)

type WhisperAudioListener struct{ modelFile string }

func (al WhisperAudioListener) OnAudioStream(e *gumble.AudioStreamEvent) {
	fmt.Println("OnAudioStream")
	fmt.Println(*e.User)
	fmt.Print("packet:")
	// fmt.Println(*<-e.C)
	// fmt.Println("--")
	go func() {
		var samples []float32
		whisperCh := make(chan []float32, 0)
		go al.audioWhisperConsumer(e.Client, whisperCh, e.User.Name)
		last := time.Now()
		drop := 0
		for {
			select {
			case pkt := <-e.C:
				start := time.Now()
				for i := 0; i < len(pkt.AudioBuffer); i += 3 /*mumble:whisper sample rate ratio*/ {
					// fmt.Println(pkt.AudioBuffer[i])
					var s float32
					for j := 0; j < 3; j++ {
						s += float32(pkt.AudioBuffer[i+j]) / 65535.0
					}
					s /= 3.0
					samples = append(samples, s)
				}
				frameSize := 3
				if len(samples) > 16000*frameSize {
					fmt.Printf("got %d000ms frame in %d ms\n", frameSize, time.Now().Sub(last).Milliseconds())
					if drop == 0 {
						whisperCh <- samples
					} else {
						drop--
					}
					samples = make([]float32, 0, 16000*10)
					last = time.Now()
					tookMs := time.Now().Sub(start).Milliseconds()
					if int(tookMs) > frameSize*1e3 {
						s := fmt.Sprintf("!!!!!! UNDERRUN: copy took %fms, budget is %ds -- dropping..", tookMs, frameSize)
						e.Client.Self.Channel.Send(s, false)
						fmt.Println(s)
						drop += int(int(tookMs) / frameSize)
					}
					fmt.Printf("copy took %s ms [%d backed up in whisperCh]\n", tookMs, len(whisperCh))
				}
			default:
				continue
			}
		}
	}()
}

func (al WhisperAudioListener) audioWhisperConsumer(client *gumble.Client, c chan []float32, username string) {
	model, err := whisper.New(al.modelFile)
	if err != nil {
		panic(err)
	}
	defer model.Close()
	for {
		frame := <-c
		start := time.Now()
		context, err := model.NewContext()
		context.SetSpeedup(true)
		fmt.Println("context.Process")
		if err = context.Process(frame, nil); err != nil {
			fmt.Fprintf(os.Stderr, "!\ncontext.Process: %s\n", err)
		}
		whisperMs := time.Now().Sub(start).Milliseconds()
		fmt.Printf("context.Process: done [%s ms]\n", whisperMs)
		for {
			segment, err := context.NextSegment()
			if err != nil {
				fmt.Fprintf(os.Stderr, "context.NextSegment: %s\n", err)
				break
			}
			caption := fmt.Sprintf("[%6s->%6s %s; w%d ms] %s\n", segment.Start, segment.End, username, whisperMs, segment.Text)
			client.Self.Channel.Send(caption, false)
			fmt.Println(caption)
		}
	}
}

// Main aids in the creation of a basic command line gumble bot. It accepts the
// following flag arguments:
//
//	--server
//	--username
//	--password
//	--insecure
//	--certificate
//	--key
func main() {
	server := flag.String("server", "localhost:64738", "Mumble server address")
	username := flag.String("username", "gumble-bot", "client username")
	password := flag.String("password", "", "client password")
	insecure := flag.Bool("insecure", false, "skip server certificate verification")
	certificateFile := flag.String("certificate", "", "user certificate file (PEM)")
	keyFile := flag.String("key", "", "user certificate key file (PEM)")
	modelFile := flag.String("model", "models/ggml-base.en.bin", "OpenAI whisper.cpp model path")

	if !flag.Parsed() {
		flag.Parse()
	}

	host, port, err := net.SplitHostPort(*server)
	if err != nil {
		host = *server
		port = strconv.Itoa(gumble.DefaultPort)
	}

	keepAlive := make(chan bool)

	config := gumble.NewConfig()
	config.Username = *username
	config.Password = *password
	address := net.JoinHostPort(host, port)

	var tlsConfig tls.Config

	if *insecure {
		tlsConfig.InsecureSkipVerify = true
	}
	if *certificateFile != "" {
		if *keyFile == "" {
			keyFile = certificateFile
		}
		if certificate, err := tls.LoadX509KeyPair(*certificateFile, *keyFile); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", os.Args[0], err)
			os.Exit(1)
		} else {
			tlsConfig.Certificates = append(tlsConfig.Certificates, certificate)
		}
	}
	config.Attach(gumbleutil.Listener{
		TextMessage: func(e *gumble.TextMessageEvent) {
			fmt.Printf("Received text message: %s\n", e.Message)
		},
	})
	config.Attach(gumbleutil.AutoBitrate)
	config.Attach(gumbleutil.Listener{
		Disconnect: func(e *gumble.DisconnectEvent) {
			keepAlive <- true
		},
		Connect: func(e *gumble.ConnectEvent) {
			fmt.Printf("Connected\n")
			cl := e.Client
			var ch *gumble.Channel
			for _, che := range cl.Channels {
				fmt.Printf("ID:%d Name:%s ()\n", che.ID, che.Name)
				if che.ID == 8 {
					ch = che
				}
			}
			if ch != nil {
				cl.Self.Move(ch)
			} else {
				fmt.Fprintf(os.Stderr, "Channel not found\n")
				os.Exit(1)
			}

		},
	})
	config.AttachAudio(WhisperAudioListener{modelFile: *modelFile})
	_, err = gumble.DialWithDialer(new(net.Dialer), address, config, &tlsConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", os.Args[0], err)
		os.Exit(1)
	}

	<-keepAlive
}

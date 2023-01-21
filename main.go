package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"layeh.com/gumble/gumble"
	"layeh.com/gumble/gumbleutil"
	_ "layeh.com/gumble/opus"
	"log"
	"net"
	"os"
	"strconv"
	// "time"

	"github.com/Jeffail/gabs/v2"
	"github.com/deepgram-devs/go-sdk/deepgram"
	"github.com/gorilla/websocket"
)

func main() {
	server := flag.String("server", "localhost:64738", "Mumble server address")
	username := flag.String("username", "gumble-bot", "client username")
	password := flag.String("password", "", "client password")
	insecure := flag.Bool("insecure", false, "skip server certificate verification")
	certificateFile := flag.String("certificate", "", "user certificate file (PEM)")
	keyFile := flag.String("key", "", "user certificate key file (PEM)")
	dgKey := flag.String("apikey", "", "deepgram API key")

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
	config.AttachAudio(TranscriptAudioListener{dgApiKey: *dgKey})
	_, err = gumble.DialWithDialer(new(net.Dialer), address, config, &tlsConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", os.Args[0], err)
		os.Exit(1)
	}

	<-keepAlive
}

type TranscriptAudioListener struct{ dgApiKey string }

func (al TranscriptAudioListener) OnAudioStream(e *gumble.AudioStreamEvent) {
	fmt.Println("OnAudioStream")
	fmt.Println(*e.User)
	fmt.Print("packet:")
	// fmt.Println(*<-e.C)
	// fmt.Println("--")
	go func() {
		var samples []byte // u16, acktually.
		transcriptCh := make(chan []byte, 0)
		go al.audioTranscriptConsumer(e.Client, transcriptCh, e.User.Name)
		// last := time.Now()
		drop := 0
		for {
			select {
			case pkt := <-e.C:
				// start := time.Now()
				for i := 0; i < len(pkt.AudioBuffer); i += 3 /*mumble:transcript sample rate ratio*/ {
					// fmt.Println(pkt.AudioBuffer[i])
					var s float32 // ugch fp slow but this isnt a µC i guess.
					for j := 0; j < 3; j++ {
						s += float32(pkt.AudioBuffer[i+j]) / 65535.0
					}
					s /= 3.0
					s -= .5
					s *= 65535.0 / 2.0
					samples = append(samples, byte(int16(s)&0xff), byte(int16(s)>>8&0xff) /*little endian*/)
				}
				frameSize := 3200 // 5hz. to be tweaked.
				if len(samples) > frameSize {
					// fmt.Printf("got %dSmp frame in %d ms\n", frameSize, time.Now().Sub(last).Milliseconds())
					if drop == 0 {
						// TODO: a /very, very/ nice to have would be some speaker detection, so we don't
						// send near-0 samples all the time and use up a bunch of credits for
						// each speaker that sent us a single sample even.
						transcriptCh <- samples
					} else {
						drop--
					}
					samples = make([]byte, 0, 16000*10*2)
					// last = time.Now()
					// tookMs := time.Now().Sub(start).Milliseconds()
					// if int(tookMs) > frameSize {
					// 	s := fmt.Sprintf("!!!!!! UNDERRUN: copy took %fms, budget is %dms -- dropping..", tookMs, frameSize)
					// 	e.Client.Self.Channel.Send(s, false)
					// 	fmt.Println(s)
					// 	drop += int(int(tookMs) / frameSize) // wrong TODO fix
					// }
					// fmt.Printf("copy took %d ms [%d backed up in transcriptCh]\n", tookMs, len(transcriptCh))
				}
			default:
				continue
			}
		}
	}()
}

func (al TranscriptAudioListener) audioTranscriptConsumer(client *gumble.Client, c chan []byte /*u8 pairs to make u16's*/, speakerName string) {
	dg := *deepgram.NewClient(al.dgApiKey)
	options := deepgram.LiveTranscriptionOptions{
		Language:  "en-US",
		Punctuate: true,
		Sample_rate: 16000,
		Channels: 1,
		Encoding: "linear16",
		Interim_results: true, // TODO: ask reese
	}

	dgConn, _, err := dg.LiveTranscription(options)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			_, message, err := dgConn.ReadMessage()
			if err != nil {
				fmt.Println("ERROR reading message")
				log.Fatal(err)
			}

			fmt.Printf("recv [raw]: %s\n", string(message))
			jsonParsed, jsonErr := gabs.ParseJSON(message)
			if jsonErr != nil {
				log.Fatal(err)
			}
			transcript := jsonParsed.Path("channel.alternatives.0.transcript").String()
			if len(transcript) > 0 {
				log.Printf("recv [transcript]: %s\n", transcript)
				client.Self.Channel.Send(fmt.Sprintf("[%s] %s", speakerName, transcript), false)
			} else {
				log.Println("recv: not sending because transcript is empty")
			}
		}
	}()

	for {
		dgConn.WriteMessage(websocket.BinaryMessage, <-c)
	}
}

package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/fhs/gompd/v2/mpd"
	"github.com/rikkuness/discord-rpc"
)

type MPDWrapper struct {
	Client *mpd.Client
}

func NewMPDWrapper() *MPDWrapper {
	return &MPDWrapper{Client: getMPDConn()}
}

func (m *MPDWrapper) Status() (mpd.Attrs, error) {
	if m.Client == nil {
		m.Client = getMPDConn()
	}
	status, err := m.Client.Status()
	if err != nil {
		log.Println("MPD connection lost, reconnecting...", err)
		m.Client.Close()
		m.Client = getMPDConn()
		status, err = m.Client.Status()
	}
	return status, err
}

func (m *MPDWrapper) CurrentSong() (mpd.Attrs, error) {
	if m.Client == nil {
		m.Client = getMPDConn()
	}
	song, err := m.Client.CurrentSong()
	if err != nil {
		log.Println("MPD connection lost while fetching song, reconnecting...", err)
		m.Client.Close()
		m.Client = getMPDConn()
		song, err = m.Client.CurrentSong()
	}
	return song, err
}

func getMPDConn() *mpd.Client {
	for {
		conn, err := mpd.Dial("tcp", "localhost:6600")
		if err != nil {
			log.Println("Failed to connect to MPD, retrying in 1s...", err)
			time.Sleep(time.Second)
			continue
		}
		return conn
	}
}

func updateActivity(rpc *discordrpc.Client, mpdClient *MPDWrapper) {
	status, err := mpdClient.Status()
	if err != nil {
		log.Println("Failed to get status:", err)
		return
	}

	song, err := mpdClient.CurrentSong()
	if err != nil {
		log.Println("Failed to get current song:", err)
		return
	}

	elapsed, _ := strconv.ParseFloat(status["elapsed"], 64)
	duration, _ := strconv.ParseFloat(status["duration"], 64)
	now := time.Now()
	start := now.Add(-time.Duration(elapsed) * time.Second)
	end := start.Add(time.Duration(duration) * time.Second)

	if status["state"] == "stop" {
		activity := discordrpc.Activity{
			Details: "Idling",
			Assets: &discordrpc.Assets{
				LargeImage: "largeimage",
				LargeText:  "Not listening anything",
			},
			Timestamps: &discordrpc.Timestamps{
				Start: &discordrpc.Epoch{Time: start},
				End:   &discordrpc.Epoch{Time: end},
			},
			Type: 2,
		}

		if err := rpc.SetActivity(activity); err != nil {
			log.Println("Could not set activity:", err)
		} else {
			fmt.Println("Updated activity:", song["Title"], "-", song["Artist"])
		}

	} else {
		activity := discordrpc.Activity{
			Details: trimForDiscord(song["Title"], 128),
			State:   trimForDiscord(song["Artist"], 128),
			Assets: &discordrpc.Assets{
				LargeImage: "largeimage",
				LargeText:  trimForDiscord(song["Album"], 128),
			},
			Timestamps: &discordrpc.Timestamps{
				Start: &discordrpc.Epoch{Time: start},
				End:   &discordrpc.Epoch{Time: end},
			},
			Type: 2,
		}

		if err := rpc.SetActivity(activity); err != nil {
			log.Println("Could not set activity:", err)
		} else {
			fmt.Println("Updated activity:", song["Title"], "-", song["Artist"])
		}
	}

}

func trimForDiscord(s string, max int) string {
	if len(s) > max {
		if max > 3 {
			return s[:max-3] + "..."
		}
		return s[:max]
	}
	return s
}

func main() {
	clientID := "1418333618530422875"

	rpc, err := discordrpc.New(clientID)
	if err != nil {
		log.Fatalf("Failed to connect to Discord RPC: %v", err)
	}

	log.Println("Connected to Discord RPC!")

	mpdClient := NewMPDWrapper()
	log.Println("Connected to MPD!")

	updateActivity(rpc, mpdClient)

	go func() {
		for {
			w, err := mpd.NewWatcher("tcp", ":6600", "")
			if err != nil {
				log.Println("Failed to create MPD watcher, retrying...", err)
				time.Sleep(time.Second)
				continue
			}

			for subsystem := range w.Event {
				if subsystem == "player" {
					updateActivity(rpc, mpdClient)
				}
			}

			w.Close()
		}
	}()

	select {}
}

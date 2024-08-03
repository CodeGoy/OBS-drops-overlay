package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	"github.com/gorilla/websocket"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	//go:embed upload.html
	uploadHTML []byte
	//go:embed control.html
	controlHTML string
	//go:embed overlay.html
	overlayHTML string
	//go:embed Icon.png
	icon                   []byte
	assetsLocationOverride string
	assetsLocation         string
	controlLink            string
	overlayLink            string
	androidGui             bool // show android gui
	mediaExt               = []string{".mp4", ".mkv", ".mp3"}
	shorten                bool
	columns                = 3
	version                = "1.6"
	ss                     = 15
)

type Server struct {
	websocketUpgrader websocket.Upgrader
	port              string
	controlChan       chan []byte
	statusChan        chan []byte
}

type Message struct {
	Action string `json:"action"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}

type FileListResponse struct {
	Files []string `json:"files"`
}

type Template struct {
	Version string
}

func shortenString(input string, length int) string {
	if !shorten {
		return input
	}
	il := len(input)
	if il >= length && il >= 10 {
		return input[:length-5] + "." + input[il-4:]
	}
	return input
}

func listDir(path string) (files []string, err error) {
	//fmt.Printf("listDir()->Path: %s\n", path)
	var pathContents []os.DirEntry
	if pathContents, err = os.ReadDir(path); err != nil {
		return nil, fmt.Errorf("Error reading dir %s: %v", path, err)
	} else {
		for _, entry := range pathContents {
			for _, ext := range mediaExt {
				if filepath.Ext(entry.Name()) == ext {
					files = append(files, entry.Name())
					break
				}
			}
		}
	}
	return
}

func (s *Server) applyTemplate(htmlString string, w http.ResponseWriter) error {
	t := template.New("t")
	if _, err := t.Parse(htmlString); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return fmt.Errorf("ERROR::t.Parse(rootHtml)::%v\n", err)
	}
	if err := t.Execute(w, Template{Version: version}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return fmt.Errorf("ERROR::t.Execute(w, &t)::%v\n", err)
	}
	return nil
}

func (s *Server) start() {
	http.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		rType := r.FormValue("type")
		fl := fmt.Sprintf("%sassets/%s/", assetsLocation, rType)
		music, err := listDir(fl)
		if err != nil {
			log.Panicf("listDir(\"%sassets/%s/\"):%v\n", assetsLocation, rType, err)
		}
		bytes, err := json.Marshal(FileListResponse{Files: music})
		if err != nil {
			log.Panicf("json.Marshal(FileListResponse{Files: %s}):%v\n", rType, err)
		}
		if _, err := w.Write(bytes); err != nil {
			log.Panicf("w.Write(bytes):%v\n", err)
		}
	})
	http.HandleFunc("/assets", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		http.ServeFile(w, r, fmt.Sprintf("%sassets/%s", assetsLocation, r.FormValue("file")))
	})
	http.HandleFunc("/uploadAsset", func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write(uploadHTML); err != nil {
			log.Panicf("%v\n", err)
		}
	})
	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		uploadLocation := r.FormValue("loc")
		fmt.Printf("/upload->Method:%s\n", r.Method)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			log.Panicf("r.FormFile(): %v\n", err)
		}
		file, handler, err := r.FormFile("file")
		if err != nil {
			log.Panicf("r.FormFile(): %v\n", err)
		}
		defer func() {
			if err := file.Close(); err != nil {
				log.Panicf("%v\v", err)
			}
		}()
		uploadTargetLocation := fmt.Sprintf("%sassets/%s/%s", assetsLocation, uploadLocation, handler.Filename)
		fmt.Println("uploadTargetLocation", uploadTargetLocation)
		f, err := os.OpenFile(uploadTargetLocation, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			log.Panicf("%v\n", err)
		}
		defer func() {
			if err := f.Close(); err != nil {
				log.Panicf("%v\v", err)
			}
		}()
		if _, err := io.Copy(f, file); err != nil {
			log.Panicf("%v\n", err)
		}
		response := fmt.Sprintf("Received File: %s", handler.Filename)
		log.Printf(response)
		if _, err := w.Write([]byte(response)); err != nil {
			log.Panicf("%v\n", err)
		}
	})
	http.HandleFunc("/control", func(w http.ResponseWriter, r *http.Request) {
		if err := s.applyTemplate(controlHTML, w); err != nil {
			log.Fatalf("%v", err)
		}
	})
	http.HandleFunc("/overlay", func(w http.ResponseWriter, r *http.Request) {
		if err := s.applyTemplate(overlayHTML, w); err != nil {
			log.Fatalf("%v", err)
		}
	})
	http.HandleFunc("/overlayWS", func(w http.ResponseWriter, r *http.Request) {
		s.websocketUpgrader.CheckOrigin = func(r *http.Request) bool { return true }
		conn, err := s.websocketUpgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("%v\n", err)
		}
		defer func() {
			fmt.Println("closing /overlayWS websocket connection")
			if err := conn.Close(); err != nil {
				log.Printf("Error closing /overlayWS connection: %v\n", err)
			}
		}()
		go func() {
			for {
				_, msg, err := conn.ReadMessage()
				if err != nil {
					log.Printf("/overlayWS: %v\n", err)
					return
				}
				s.statusChan <- msg
			}
		}()
		for {
			if err = conn.WriteMessage(1, <-s.controlChan); err != nil {
				return
			}
		}
	})
	http.HandleFunc("/controlWS", func(w http.ResponseWriter, r *http.Request) {
		s.websocketUpgrader.CheckOrigin = func(r *http.Request) bool { return true }
		conn, err := s.websocketUpgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("%v\n", err)
		}
		defer func() {
			fmt.Println("closing /controlWS websocket connection")
			if err := conn.Close(); err != nil {
				log.Printf("Error closing /controlWS connection: %v\n", err)
			}
		}()
		go func() {
			for {
				_, msg, err := conn.ReadMessage()
				if err != nil {
					log.Printf("/controlWS: %v\n", err)
					return
				}
				s.controlChan <- msg
			}
		}()
		for {
			if err = conn.WriteMessage(1, <-s.statusChan); err != nil {
				return
			}
		}
	})
	log.Printf("Serving @ %s %s\n", overlayLink, controlLink)
	if err := http.ListenAndServe(":"+s.port, nil); err != nil {
		log.Printf("Error:: listenAndServe():: %v\n", err)
	}
}

func (s *Server) sendOverlayMessage(action, key, value string) {
	js, err := json.Marshal(Message{
		Action: action,
		Key:    key,
		Value:  value,
	})
	if err != nil {
		log.Printf("Error:: Failed to marshal json:: %v\n", err)
	}
	s.controlChan <- js
}

func (s *Server) controlsGUI(a fyne.App) {
	w := a.NewWindow("controls")
	w.CenterOnScreen()
	if runtime.GOOS == "android" {
		baseFilePath := "/storage/self/primary/Android" // TODO : find a programmatic way to get this string
		assetsLocation = baseFilePath + strings.Replace(strings.Replace(a.Storage().RootURI().Path(), "user/0/", "", -1), "fyne", "", -1)
		fmt.Println("assetsLocation:", assetsLocation)
		for _, endPAth := range []string{"assets/drops", "assets/music"} {
			mkdirPAth := assetsLocation + endPAth
			if err := os.MkdirAll(mkdirPAth, os.ModePerm); err != nil {
				log.Panicf("Failed to create dir: %v\n", err)
			}
		}
	} else {
		w.Resize(fyne.NewSize(800, 800))
	}
	// TODO : add all controls...
	ng := container.NewVBox()
	contentGrid := container.NewHBox()
	audioDropsGrid := container.NewGridWithColumns(columns)
	videoDropsGrid := container.NewGridWithColumns(columns)
	musicGrid := container.NewGridWithColumns(columns)
	contentGrid.Add(widget.NewButton("exit", func() {
		os.Exit(0)
	}))
	dropsDirContents, err := listDir(assetsLocation + "assets/drops")
	if err != nil {
		log.Panicf("Failed to list dir: %v\n", err)
	}
	audioDropsGrid.Add(widget.NewButton("stop audio", func() {
		go s.sendOverlayMessage("audio", "stop", "audio")
	}))
	for _, f := range dropsDirContents {
		if strings.Contains(f, ".mp3") {
			audioDropsGrid.Add(widget.NewButton(shortenString(f[:len(f)-4], ss), func() {
				go s.sendOverlayMessage("audio", "play", fmt.Sprintf("drops/%s", f))
			}))
		}
	}
	videoDropsGrid.Add(widget.NewButton("stop video", func() {
		go s.sendOverlayMessage("video", "stop", "video")
	}))
	for _, f := range dropsDirContents {
		if strings.Contains(f, ".mp4") || strings.Contains(f, ".mkv") {
			videoDropsGrid.Add(widget.NewButton(shortenString(f[:len(f)-4], ss), func() {
				go s.sendOverlayMessage("video", "play", fmt.Sprintf("drops/%s", f))
			}))
		}
	}
	musicDirContents, err := listDir(assetsLocation + "assets/music")
	if err != nil {
		log.Panicf("Failed to list dir: %v\n", err)
	}
	musicGrid.Add(widget.NewButton("stop music", func() {
		go s.sendOverlayMessage("music", "stop", "music")
	}))
	for _, f := range musicDirContents {
		if strings.Contains(f, ".mp3") {
			musicGrid.Add(widget.NewButton(shortenString(f[:len(f)-4], ss), func() {
				go s.sendOverlayMessage("music", "play", fmt.Sprintf("music/%s", f))
			}))
		}
	}
	ng.Add(contentGrid)
	ng.Add(widget.NewLabel("Audio Drops"))
	ng.Add(audioDropsGrid)
	ng.Add(widget.NewLabel("Video Drops"))
	ng.Add(videoDropsGrid)
	ng.Add(widget.NewLabel("Music"))
	ng.Add(musicGrid)
	w.SetContent(container.NewVScroll(ng))
	w.Show()
}

func (s *Server) gui() {
	a := app.NewWithID("codegoy.obs.overlay")
	version = a.Metadata().Version
	a.SetIcon(fyne.NewStaticResource("icon", icon))
	if androidGui {
		s.controlsGUI(a)
		a.Run()
	} else if desk, ok := a.(desktop.App); ok {
		a.SetIcon(fyne.NewStaticResource("icon", icon))
		w := a.NewWindow(fmt.Sprintf("OBS-drops-overlay v%s", version))
		m := fyne.NewMenu("links",
			fyne.NewMenuItem("Links", func() {
				w.Show()
			}),
			fyne.NewMenuItem("Controls", func() {
				s.controlsGUI(a)
			}),
		)
		desk.SetSystemTrayMenu(m)
		ng := container.NewAdaptiveGrid(1)
		ng.Add(widget.NewLabel(controlLink))
		ng.Add(widget.NewLabel(overlayLink))
		w.SetContent(ng)
		w.SetCloseIntercept(func() {
			w.Hide()
		})
		a.Run()
	}
}

func main() {
	var port string
	var enableGui bool
	flag.StringVar(&port, "port", "8605", "port to listen on")
	flag.StringVar(&assetsLocationOverride, "path", "", "override file location")
	flag.BoolVar(&enableGui, "gui", false, "show gui non windows os")
	flag.BoolVar(&androidGui, "dev", false, "show android gui on desktop (for dev)")
	flag.BoolVar(&shorten, "short", false, "shorten button labels(makes android app look better)")
	flag.Parse()
	var ip string
	if runtime.GOOS == "android" {
		enableGui, androidGui, shorten = true, true, true
		ip = "127.0.0.1" // TODO : get LocalIP from ConnectionManager in android
		columns = 2
	} else {
		ip = func() string {
			adders, err := net.InterfaceAddrs()
			if err != nil {
				log.Panicf("net.InterfaceAddrs:%v\n", err)
			}
			for _, address := range adders {
				if inet, ok := address.(*net.IPNet); ok && !inet.IP.IsLoopback() {
					fmt.Printf("Network Interface: %v %s\n", inet.IP, inet.String())
					if inet.IP.To4() != nil {
						return inet.IP.String()
					}
				}
			}
			return ""
		}()
	}
	if runtime.GOOS == "windows" {
		enableGui = true
	}
	controlLink = fmt.Sprintf("http://%s:%s/control", ip, port)
	overlayLink = fmt.Sprintf("http://%s:%s/overlay", ip, port)
	s := Server{
		websocketUpgrader: websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024},
		port:              port,
		controlChan:       make(chan []byte),
		statusChan:        make(chan []byte),
	}
	if assetsLocationOverride != "" {
		if assetsLocationOverride[len(assetsLocationOverride)-1:] != "/" {
			assetsLocationOverride += "/"
		}
		assetsLocation = assetsLocationOverride
	}
	log.Printf("Starting obs-drops-overlay v%s\n", version)
	if enableGui {
		go func() {
			s.start()
		}()
		s.gui()
	} else {
		s.start()
	}
}
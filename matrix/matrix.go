// designed to interact with https://github.com/lolarobins/ESP32-Matrix-Controller
package matrix

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"image"
	"image/draw"
	"image/gif"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fogleman/gg"
	"github.com/nfnt/resize"
)

type MatrixPanel struct {
	// name of the panel
	Name string `json:"name"`
	// address of the matrix display
	Address string `json:"address"`
	// width of the panel
	Width uint8 `json:"width"`
	// height of the panel
	Height uint8 `json:"height"`
	// context used when calling draw
	Context *gg.Context `json:"-"`
	// store own id
	Id string `json:"-"`

	// animation handling
	animation bool
	animmutex *sync.Mutex
}

var Panels = make(map[string]*MatrixPanel)

func LoadPanels() error {
	// load panels
	if info, err := os.Stat("panels"); !os.IsNotExist(err) && info.IsDir() {
		files, err := os.ReadDir("panels")
		if err != nil {
			return err
		}

		for _, f := range files {
			// ignore dotfiles, non-directories
			if strings.HasPrefix(f.Name(), ".") || f.IsDir() {
				continue
			}

			data, err := os.ReadFile("panels/" + f.Name())
			if err != nil {
				println(f.Name() + ": " + err.Error())
			}

			// define and set id
			panel := new(MatrixPanel)
			id := strings.TrimSuffix(f.Name(), ".json")
			panel.Id = id

			if err := json.Unmarshal(data, panel); err != nil {
				println(id + " panel: " + err.Error())
			}

			if err := panel.SaveConfig(); err != nil {
				println(id + " panel: " + err.Error())
			}

			// non-json defaults
			panel.Context = gg.NewContext(int(panel.Width), int(panel.Height))
			panel.animmutex = &sync.Mutex{}

			println("Loaded panel " + id)
			Panels[id] = panel
		}
	} else if !os.IsNotExist(err) {
		return err
	} else if err := os.Mkdir("panels", 0777); err != nil {
		return err
	}

	return nil
}

func NewPanel(id string, address string, w uint8, h uint8) (MatrixPanel, error) {
	panel := MatrixPanel{
		Address:   address,
		Width:     w,
		Height:    h,
		Context:   gg.NewContext(int(w), int(h)),
		Id:        id,
		animmutex: &sync.Mutex{},
	}

	if err := panel.SaveConfig(); err != nil {
		return panel, err
	}

	return panel, nil
}

func (m *MatrixPanel) SaveConfig() error {
	data, err := json.MarshalIndent(m, "", "    ")
	if err != nil {
		return errors.New("error marshalling JSON to output to file")
	}

	if err := os.WriteFile("panels/"+m.Id+".json", data, 0777); err != nil {
		return errors.New("could not write to file 'panels/" + m.Id + ".json'")
	}

	return nil
}

func (m *MatrixPanel) FillImage(filepath string) error {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer(data)
	var img image.Image

	split := strings.Split(filepath, ".")

	switch strings.ToLower(split[len(split)-1]) { // extension getting

	case "gif":
		var gifimg *gif.GIF
		if gifimg, err = gif.DecodeAll(buf); err != nil {
			return err
		}

		m.RenderGIF(*gifimg)
		return nil
	default:
		m.animation = false // stop animation and wait for it to finish
		m.animmutex.Lock()
		if img, _, err = image.Decode(buf); err != nil {
			return err
		}
	}

	resized := resize.Resize(uint(m.Width), uint(m.Height), img, resize.Bilinear)
	m.Context.DrawImage(resized, 0, 0) // resize to fit display

	err = m.Draw()

	m.animmutex.Unlock()

	return err
}

func getGifDimensions(gif *gif.GIF) (x int, y int) {
	var lowestX int
	var lowestY int
	var highestX int
	var highestY int

	for _, img := range gif.Image {
		if img.Rect.Min.X < lowestX {
			lowestX = img.Rect.Min.X
		}
		if img.Rect.Min.Y < lowestY {
			lowestY = img.Rect.Min.Y
		}
		if img.Rect.Max.X > highestX {
			highestX = img.Rect.Max.X
		}
		if img.Rect.Max.Y > highestY {
			highestY = img.Rect.Max.Y
		}
	}

	return highestX - lowestX, highestY - lowestY
}

func (m *MatrixPanel) RenderGIF(img gif.GIF) {
	// decode frames https://stackoverflow.com/questions/33295023/how-to-split-gif-into-images
	m.animation = false
	go func() {
		m.animmutex.Lock()
		m.animation = true

		// decode frames (takes a seconding)
		width, height := getGifDimensions(&img)
		images := make([]*image.RGBA, len(img.Image))

		for i := 0; i < len(img.Image); i++ {
			images[i] = image.NewRGBA(image.Rect(0, 0, width, height))
			draw.Draw(images[i], images[i].Bounds(), img.Image[0], image.Point{X: 0, Y: 0}, draw.Src)

			for j := 0; j < i; j++ {
				draw.Draw(images[i], images[i].Bounds(), img.Image[j], image.Point{X: 0, Y: 0}, draw.Over)
			}
		}

		var current, last int64
		for i, loops := 0, 0; m.animation; {
			current = time.Now().UnixMilli()
			if i != 0 && (current-last) > int64(img.Delay[i-1]*10) && last != 0 {
				continue
			}

			if i == len(img.Image) {
				i = 0
				if img.LoopCount == loops && img.LoopCount != 0 {
					m.animation = false
				}

				loops++
			}

			resized := resize.Resize(uint(m.Width), uint(m.Height), images[i], resize.Lanczos3)
			m.Context.DrawImage(resized, 0, 0) // resize to fit display

			m.Draw()

			last = time.Now().UnixMilli()
			i++
		}

		m.animmutex.Unlock()
	}()
}

// draw the current canvas to the screen
func (m *MatrixPanel) Draw() error {
	imgdata := make([]byte, (uint32(m.Width)*uint32(m.Height))*2) // w * h * 2 bytes each

	for x := 0; x < int(m.Width); x++ { // convert and fill array
		for y := 0; y < int(m.Height); y++ {
			r888, g888, b888, _ := m.Context.Image().At(x, y).RGBA()

			r565 := ((r888 >> 3) & 0x1f) << 11
			g565 := ((g888 >> 2) & 0x3f) << 5
			b565 := (b888 >> 3) & 0x1f

			vals := make([]byte, 2)
			binary.LittleEndian.PutUint16(vals, uint16(r565|g565|b565))

			imgdata[((y*64)+x)*2] = vals[0]
			imgdata[(((y*64)+x)*2)+1] = vals[1]
		}
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "server-upload.bin")
	io.Copy(part, bytes.NewBuffer(imgdata))
	writer.Close()

	resp, err := http.Post("http://"+m.Address+"/upload", writer.FormDataContentType(), body)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New("draw returned non-OK status code")
	}

	return nil
}

// clears the screen of the panel
func (m *MatrixPanel) Clear() error {
	resp, err := http.Get("http://" + m.Address + "/clear")

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return errors.New("clear returned non-OK status code")
	}

	return nil
}
